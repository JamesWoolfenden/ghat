'use strict';

const vscode = require('vscode');
const { LanguageClient } = require('vscode-languageclient/node');

let client;

function activate(context) {
  const config = vscode.workspace.getConfiguration('ghat');
  const binary = config.get('path', 'ghat');
  const token = config.get('githubToken', '');

  const args = ['lsp'];
  if (token) {
    args.push('--token', token);
  }

  const serverOptions = { command: binary, args };

  const clientOptions = {
    documentSelector: [
      { scheme: 'file', pattern: '**/.github/workflows/*.{yml,yaml}' },
      { scheme: 'file', pattern: '**/action.{yml,yaml}' },
      { scheme: 'file', pattern: '**/go.mod' },
      { scheme: 'file', pattern: '**/package.json' },
      { scheme: 'file', pattern: '**/requirements*.txt' },
      { scheme: 'file', pattern: '**/Cargo.toml' },
      { scheme: 'file', pattern: '**/Gemfile' },
      { scheme: 'file', pattern: '**/.pre-commit-config.{yaml,yml}' },
      { scheme: 'file', pattern: '**/cpanfile' },
      { scheme: 'file', language: 'dockerfile' },
      { scheme: 'file', pattern: '**/Dockerfile' },
      { scheme: 'file', pattern: '**/Dockerfile.*' },
      { scheme: 'file', pattern: '**/.gitlab-ci.{yml,yaml}' },
      { scheme: 'file', pattern: '**/*.gitlab-ci.{yml,yaml}' },
      { scheme: 'file', pattern: '**/docker-compose.{yml,yaml}' },
      { scheme: 'file', pattern: '**/compose.{yml,yaml}' },
      { scheme: 'file', pattern: '**/*.tf' },
      { scheme: 'file', pattern: '**/*.yaml' },
      { scheme: 'file', pattern: '**/*.yml' },
    ],
  };

  client = new LanguageClient('ghat', 'ghat', serverOptions, clientOptions);
  client.start();
  context.subscriptions.push(client);
}

function deactivate() {
  return client?.stop();
}

module.exports = { activate, deactivate };
