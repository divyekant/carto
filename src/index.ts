#!/usr/bin/env node
import { Command } from 'commander';

const program = new Command();

program
  .name('codex')
  .description('Codebase intelligence - index any codebase into a layered context graph')
  .version('0.1.0');

program
  .command('index')
  .description('Index a codebase')
  .argument('<path>', 'Path to the codebase to index')
  .option('--full', 'Force full re-index (ignore cache)')
  .option('--layers <layers>', 'Comma-separated layers to run (0,1,2,3,4)', '0,1,2,3,4')
  .option('--dry-run', 'Show what would be indexed without making LLM calls')
  .action(async (path: string, options) => {
    console.log(`Indexing ${path}...`);
  });

program
  .command('query')
  .description('Query the indexed codebase')
  .argument('<query>', 'Natural language query')
  .option('--layer <layer>', 'Filter by layer (0-4)')
  .option('--domain <domain>', 'Filter by domain name')
  .option('-k <count>', 'Number of results', '10')
  .action(async (query: string, options) => {
    console.log(`Querying: ${query}`);
  });

program
  .command('patterns')
  .description('Generate pattern enforcement files (CLAUDE.md, .cursorrules)')
  .argument('<path>', 'Path to the codebase')
  .option('--format <format>', 'Output format: claude, cursor, all', 'all')
  .action(async (path: string, options) => {
    console.log(`Generating patterns for ${path}...`);
  });

program
  .command('status')
  .description('Show index status for a codebase')
  .argument('<path>', 'Path to the codebase')
  .action(async (path: string) => {
    console.log(`Status for ${path}...`);
  });

program.parse();
