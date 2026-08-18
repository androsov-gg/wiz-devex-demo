FROM codercom/code-server:latest

USER root

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    git \
    jq \
    ca-certificates \
    sudo \
    nodejs \
    npm \
    && rm -rf /var/lib/apt-get/lists/*

# Switch to user coder for all remaining installations, configurations, and runtime
USER coder
ENV PATH="/home/coder/.local/bin:${PATH}"

# Configure npm prefix so global npm packages install into user directory ~/.local
RUN mkdir -p /home/coder/.local/bin \
    /home/coder/.local/share/code-server/User \
    /home/coder/.claude/commands \
    /home/coder/project \
    && npm config set prefix '/home/coder/.local'

# Install Wiz CLI into user local bin (~/.local/bin/wizcli)
RUN curl -sSfL https://downloads.wiz.io/v1/wizcli/latest/wizcli-linux-amd64 -o /home/coder/.local/bin/wizcli \
    && chmod +x /home/coder/.local/bin/wizcli

# Copy user configurations with coder ownership
COPY --chown=coder:coder vscode_settings.json /home/coder/.local/share/code-server/User/settings.json
COPY --chown=coder:coder claude-code/settings.json /home/coder/.claude/settings.json
COPY --chown=coder:coder claude-code/CLAUDE.md /home/coder/.claude/CLAUDE.md
COPY --chown=coder:coder claude-code/commands/ /home/coder/.claude/commands/
COPY --chown=coder:coder claude-code/.claude.json /home/coder/.claude.json

# Install Claude Code CLI into user local bin (~/.local/bin/claude)
RUN npm install -g @anthropic-ai/claude-code || true

# Pre-install VS Code extensions into code-server
RUN code-server --install-extension WizCloud.wiz-vscode || true \
    && code-server --install-extension anthropic.claude-code || true

# Copy entrypoint script into user local bin (~/.local/bin/entrypoint.sh)
COPY --chmod=755 --chown=coder:coder entrypoint.sh /home/coder/.local/bin/entrypoint.sh

WORKDIR /home/coder

ENTRYPOINT ["/home/coder/.local/bin/entrypoint.sh"]