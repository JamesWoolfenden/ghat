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
      { scheme: 'file', pattern: '**/.gitlab-ci.{yml,yaml}' },
      { scheme: 'file', pattern: '**/.pre-commit-config.{yaml,yml}' },
      { scheme: 'file', language: 'dockerfile' },
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
