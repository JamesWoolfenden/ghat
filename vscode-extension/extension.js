const { LanguageClient, TransportKind } = require('vscode-languageclient/node');

let client;

function activate(context) {
    const serverOptions = {
        command: 'ghat',
        args: ['--quiet', 'lsp'],
        transport: TransportKind.stdio,
    };
    const clientOptions = {
        documentSelector: [
            { scheme: 'file', pattern: '**/.github/workflows/*.{yml,yaml}' },
            { scheme: 'file', pattern: '**/go.mod' },
            { scheme: 'file', pattern: '**/package.json' },
            { scheme: 'file', pattern: '**/requirements*.txt' },
            { scheme: 'file', pattern: '**/Cargo.toml' },
            { scheme: 'file', pattern: '**/Gemfile' },
            { scheme: 'file', pattern: '**/.pre-commit-config.{yml,yaml}' },
            { scheme: 'file', pattern: '**/cpanfile' },
            { scheme: 'file', pattern: '**/Dockerfile' },
            { scheme: 'file', pattern: '**/Dockerfile.*' },
            { scheme: 'file', pattern: '**/.gitlab-ci.yml' },
            { scheme: 'file', pattern: '**/*.gitlab-ci.yml' },
            { scheme: 'file', pattern: '**/docker-compose.{yml,yaml}' },
            { scheme: 'file', pattern: '**/compose.{yml,yaml}' },
            { scheme: 'file', pattern: '**/*.tf' },
            { scheme: 'file', pattern: '**/*.yaml' },
            { scheme: 'file', pattern: '**/*.yml' },
        ],
    };
    client = new LanguageClient('ghat-lsp', 'ghat', serverOptions, clientOptions);
    client.start();
}

function deactivate() {
    if (client) return client.stop();
}

module.exports = { activate, deactivate };
