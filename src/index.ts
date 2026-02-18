#!/usr/bin/env node
import { Command } from 'commander';
import { resolve, basename } from 'path';
import Anthropic from '@anthropic-ai/sdk';
import chalk from 'chalk';
import ora from 'ora';
import { loadConfig } from './config.js';
import { FaissClient } from './faiss-client.js';
import { Scanner } from './scanner.js';
import { Layer1Analyzer } from './analyzers/layer1.js';
import { DeepAnalyzer } from './analyzers/deep.js';
import { CodexStorage } from './storage.js';
import { IndexManifest } from './manifest.js';
import { SkillGenerator } from './skill-generator.js';
import { LlmClient } from './llm.js';
import { Pipeline } from './pipeline.js';
import { isOAuthToken, makeOAuthFetch } from './oauth.js';
import { writeFileSync } from 'fs';
import { join } from 'path';

const program = new Command();

program
  .name('codex')
  .description('Codebase intelligence - index any codebase into a layered context graph')
  .version('0.1.0');

program
  .command('index')
  .description('Index a codebase')
  .argument('<path>', 'Path to the codebase to index')
  .option('--full', 'Force full re-index')
  .action(async (targetPath: string, options) => {
    const absPath = resolve(targetPath);
    const projectName = basename(absPath);
    const config = loadConfig();

    const spinner = ora('Checking FAISS connection...').start();

    const faiss = new FaissClient(config.faissUrl, config.faissApiKey);
    const healthy = await faiss.health();
    if (!healthy) {
      spinner.fail('Cannot connect to FAISS at ' + config.faissUrl);
      process.exit(1);
    }

    if (!config.anthropicApiKey) {
      spinner.fail('ANTHROPIC_API_KEY not set');
      process.exit(1);
    }

    let anthropic: Anthropic;
    if (isOAuthToken(config.anthropicApiKey)) {
      spinner.text = 'Using OAuth token...';
      anthropic = new Anthropic({
        apiKey: 'placeholder',
        fetch: makeOAuthFetch(config.anthropicApiKey),
      } as any);
    } else {
      anthropic = new Anthropic({ apiKey: config.anthropicApiKey });
    }
    const llm = new LlmClient(anthropic, {
      maxConcurrent: config.maxConcurrentLlmCalls,
      haikuModel: config.haikuModel,
      opusModel: config.opusModel,
    });

    const scanner = new Scanner(absPath);
    const layer1 = new Layer1Analyzer(llm);
    const deep = new DeepAnalyzer(llm);
    const storage = new CodexStorage(faiss, projectName);
    const manifest = new IndexManifest(absPath, projectName);
    const skillGenerator = new SkillGenerator();

    if (!options.full) manifest.load();

    const pipeline = new Pipeline({
      scanner, layer1, deep, storage, manifest, skillGenerator, projectPath: absPath,
      log: (msg) => {
        if (msg.startsWith('Phase')) {
          spinner.text = msg;
        } else {
          spinner.stop();
          console.log(chalk.dim(msg));
          spinner.start();
        }
      },
    });

    try {
      const result = await pipeline.runFull();

      spinner.succeed(chalk.green('Indexing complete!'));
      console.log('');
      console.log(`  Files scanned:  ${chalk.bold(String(result.filesProcessed))}`);
      console.log(`  Code units:     ${chalk.bold(String(result.unitsGenerated))}`);
      console.log(`  Domains found:  ${chalk.bold(String(result.domainsFound))}`);
      console.log(`  Languages:      ${result.scan.languages.join(', ')}`);
      console.log('');
      console.log(`Run ${chalk.cyan('codex query "your question"')} to search the index.`);
      console.log(`Run ${chalk.cyan('codex patterns ' + targetPath)} to generate CLAUDE.md.`);
    } catch (err) {
      spinner.fail('Indexing failed');
      console.error(err);
      process.exit(1);
    }
  });

program
  .command('query')
  .description('Query the indexed codebase')
  .argument('<query>', 'Natural language query')
  .option('--project <name>', 'Project name to search within')
  .option('-k <count>', 'Number of results', '10')
  .action(async (query: string, options) => {
    const config = loadConfig();
    const faiss = new FaissClient(config.faissUrl, config.faissApiKey);

    try {
      const results = await faiss.search(query, {
        k: parseInt(options.k),
      });

      if (results.length === 0) {
        console.log(chalk.yellow('No results found. Have you indexed a project?'));
        return;
      }

      for (const result of results) {
        const score = (result.score * 100).toFixed(1);
        console.log(chalk.dim(`[${score}%]`) + ' ' + chalk.cyan(result.source));
        console.log(result.text.slice(0, 300));
        console.log('');
      }
    } catch (err) {
      console.error(chalk.red('Query failed:'), err);
      process.exit(1);
    }
  });

program
  .command('patterns')
  .description('Generate pattern enforcement files')
  .argument('<path>', 'Path to the indexed codebase')
  .option('--format <format>', 'Output format: claude, cursor, all', 'all')
  .action(async (targetPath: string, options) => {
    const absPath = resolve(targetPath);
    const projectName = basename(absPath);
    const config = loadConfig();
    const faiss = new FaissClient(config.faissUrl, config.faissApiKey);

    const spinner = ora('Fetching indexed patterns...').start();

    try {
      const patternResults = await faiss.search('coding patterns conventions naming', { k: 5 });
      const domainResults = await faiss.search('domain bounded context', { k: 20 });
      const systemResults = await faiss.search('system overview architecture', { k: 3 });

      if (patternResults.length === 0 && systemResults.length === 0) {
        spinner.fail('No indexed data found. Run `codex index` first.');
        return;
      }

      spinner.succeed('Patterns fetched');

      if (options.format === 'claude' || options.format === 'all') {
        const claudeMdPath = join(absPath, 'CLAUDE.md');
        const content = `# Codebase Intelligence (Auto-generated by Codex)\n\n${patternResults.map(r => r.text).join('\n\n')}\n\n## System\n\n${systemResults.map(r => r.text).join('\n\n')}\n\n## Domains\n\n${domainResults.map(r => r.text).join('\n\n')}`;
        writeFileSync(claudeMdPath, content);
        console.log(chalk.green(`  Generated ${claudeMdPath}`));
      }

      if (options.format === 'cursor' || options.format === 'all') {
        const cursorPath = join(absPath, '.cursorrules');
        const content = `# Project Conventions (Auto-generated by Codex)\n\n${patternResults.map(r => r.text).join('\n\n')}`;
        writeFileSync(cursorPath, content);
        console.log(chalk.green(`  Generated ${cursorPath}`));
      }
    } catch (err) {
      spinner.fail('Failed to fetch patterns');
      console.error(err);
      process.exit(1);
    }
  });

program
  .command('status')
  .description('Show index status')
  .argument('<path>', 'Path to the codebase')
  .action(async (targetPath: string) => {
    const absPath = resolve(targetPath);
    const projectName = basename(absPath);
    const manifest = new IndexManifest(absPath, projectName);

    if (manifest.isFirstRun()) {
      console.log(chalk.yellow('This project has not been indexed yet.'));
      console.log(`Run ${chalk.cyan('codex index ' + targetPath)} to index it.`);
      return;
    }

    manifest.load();
    console.log(chalk.bold('Codex Index Status'));
    console.log('');
    console.log(`  Project: ${chalk.cyan(projectName)}`);
    console.log(`  Path:    ${absPath}`);
    console.log('');
    console.log(`Run ${chalk.cyan('codex index ' + targetPath)} to re-index.`);
  });

program.parse();
