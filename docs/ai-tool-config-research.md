# AI Coding Tool Configuration Research

Research findings on how popular AI coding CLI tools store and configure API endpoints for redirecting to custom OpenAI-compatible proxies.

---

## 1. Aider

### Config Location
- **Primary**: `.aider.conf.yml` (searched in order: home dir → git root → current dir)
- **Alternative**: `--config <filename>` flag to specify custom location
- **Environment**: `.env` file in git root (via `--env-file` flag)

### Config Format
- **Format**: YAML
- **Example**:
```yaml
# .aider.conf.yml
openai-api-base: http://localhost:20199/v1
openai-api-key: your-key-here
model: gpt-4
```

### API Endpoint Override
- **Field**: `openai-api-base`
- **CLI Flag**: `--openai-api-base VALUE`
- **Environment Variable**: `AIDER_OPENAI_API_BASE`
- **Description**: Specify the API base URL

### Authentication Methods
- **API Keys**: 
  - `openai-api-key` (YAML or env var `AIDER_OPENAI_API_KEY`)
  - `anthropic-api-key` (YAML or env var `AIDER_ANTHROPIC_API_KEY`)
  - Generic: `--api-key PROVIDER=KEY` (sets `PROVIDER_API_KEY` env var)
- **Environment Variables**: `--set-env ENV_VAR_NAME=value`

### Additional Settings
- `openai-api-type`: API type (deprecated, use `--set-env OPENAI_API_TYPE=`)
- `openai-api-version`: API version (deprecated)
- `openai-api-deployment-id`: Deployment ID for Azure (deprecated)
- `openai-organization-id`: Organization ID (deprecated)
- `verify-ssl`: Enable/disable SSL verification (default: true)
- `timeout`: Timeout in seconds for API calls

### Example: Redirect to Custom Proxy
```yaml
# .aider.conf.yml
openai-api-base: http://localhost:20199/v1
model: gpt-4
verify-ssl: false  # if using self-signed cert
```

```bash
# CLI
aider --openai-api-base http://localhost:20199/v1 --model gpt-4

# Environment variable
export AIDER_OPENAI_API_BASE=http://localhost:20199/v1
aider
```

---

## 2. Continue.dev

### Config Location
- **VS Code**: `~/.continue/config.json` (Linux/macOS) or `%USERPROFILE%\.continue\config.json` (Windows)
- **JetBrains**: `~/.continue/config.json` (same location)
- **Format**: JSON

### Config Format
```json
{
  "models": [
    {
      "title": "GPT-4",
      "provider": "openai",
      "model": "gpt-4",
      "apiBase": "http://localhost:20199/v1",
      "apiKey": "your-key-here"
    }
  ]
}
```

### API Endpoint Override
- **Field**: `apiBase` (within model configuration)
- **Alternative**: `baseURL` (some providers)
- **Description**: Custom API endpoint for OpenAI-compatible servers

### Authentication Methods
- **API Key**: `apiKey` field in model config
- **Environment Variables**: Can reference env vars in config

### Provider Types
- `openai` - OpenAI API
- `anthropic` - Anthropic API
- `ollama` - Local Ollama
- `lmstudio` - LM Studio
- `openai-compatible` - Generic OpenAI-compatible APIs

### Example: Redirect to Custom Proxy
```json
{
  "models": [
    {
      "title": "Custom Proxy GPT-4",
      "provider": "openai",
      "model": "gpt-4",
      "apiBase": "http://localhost:20199/v1",
      "apiKey": "sk-custom-key"
    }
  ],
  "tabAutocompleteModel": {
    "title": "Custom Proxy Codex",
    "provider": "openai",
    "model": "gpt-3.5-turbo",
    "apiBase": "http://localhost:20199/v1",
    "apiKey": "sk-custom-key"
  }
}
```

---

## 3. Cursor IDE

### Config Location
- **Settings**: Accessible via IDE settings UI (Cursor Settings → Models)
- **Config File**: Not publicly documented (proprietary)
- **Platform-specific**:
  - macOS: `~/Library/Application Support/Cursor/User/settings.json`
  - Windows: `%APPDATA%\Cursor\User\settings.json`
  - Linux: `~/.config/Cursor/User/settings.json`

### Config Format
- **Format**: JSON (VS Code-compatible settings)
- **Note**: Cursor uses VS Code's settings format but with custom extensions

### API Endpoint Override
- **Method**: Not officially documented for custom endpoints
- **Workaround**: May support OpenAI-compatible base URL override via settings
- **Field** (unconfirmed): Potentially `cursor.apiBase` or similar

### Authentication Methods
- **Built-in**: Cursor account authentication (primary)
- **API Keys**: Can configure OpenAI/Anthropic keys in settings
- **Environment Variables**: May respect `OPENAI_API_BASE` and `OPENAI_API_KEY`

### Example: Potential Configuration
```json
// settings.json (unconfirmed)
{
  "cursor.apiBase": "http://localhost:20199/v1",
  "cursor.apiKey": "your-key-here"
}
```

**Note**: Cursor's configuration is not fully documented for custom proxy usage. Community reports suggest limited support for custom endpoints.

---

## 4. GitHub Copilot CLI

### Config Location
- **CLI Config**: `gh config` command
- **Config File**: `~/.config/gh/config.yml` (Linux/macOS) or `%APPDATA%\GitHub CLI\config.yml` (Windows)
- **Format**: YAML

### Config Format
```yaml
# config.yml
git_protocol: https
editor: vim
prompt: enabled
pager: less
```

### API Endpoint Override
- **Method**: Not officially supported for custom endpoints
- **Limitation**: GitHub Copilot is tightly integrated with GitHub's infrastructure
- **Environment Variables**: 
  - `GITHUB_TOKEN` - GitHub authentication token
  - `GH_HOST` - GitHub Enterprise Server hostname (not for API proxy)

### Authentication Methods
- **OAuth**: `gh auth login` (primary method)
- **Token**: `GITHUB_TOKEN` environment variable
- **Enterprise**: `gh auth login --hostname <enterprise-host>`

### Configuration Commands
```bash
# List all config
gh config list

# Get specific config
gh config get editor

# Set config
gh config set editor vim

# Clear cache
gh config clear-cache
```

### Example: Limited Customization
```bash
# GitHub Copilot uses GitHub's API exclusively
# No official way to redirect to custom proxy

# Environment variables (limited effect)
export GITHUB_TOKEN=ghp_xxxxx
export GH_HOST=github.example.com  # Enterprise only
```

**Note**: GitHub Copilot CLI does not support custom OpenAI-compatible proxies. It requires GitHub authentication and uses GitHub's Copilot API exclusively.

---

## Summary Table

| Tool | Config Location | Format | API Base Field | Env Var Override | Custom Proxy Support |
|------|----------------|--------|----------------|------------------|---------------------|
| **Aider** | `.aider.conf.yml` | YAML | `openai-api-base` | `AIDER_OPENAI_API_BASE` | ✅ Full support |
| **Continue.dev** | `~/.continue/config.json` | JSON | `apiBase` | N/A (in config) | ✅ Full support |
| **Cursor** | `settings.json` | JSON | Undocumented | `OPENAI_API_BASE` (maybe) | ⚠️ Limited/undocumented |
| **GitHub Copilot** | `~/.config/gh/config.yml` | YAML | Not supported | `GH_HOST` (enterprise only) | ❌ No support |

---

## Recommendations for dntproxy

### 1. Target Tools with Full Support
- **Aider**: Excellent candidate - well-documented, flexible configuration
- **Continue.dev**: Excellent candidate - JSON config, clear documentation

### 2. Provide Setup Instructions
Create tool-specific setup guides:

#### Aider Setup
```bash
# Option 1: YAML config
cat > ~/.aider.conf.yml <<EOF
openai-api-base: http://localhost:20199/v1
model: kr/claude-sonnet-4.5
EOF

# Option 2: Environment variable
export AIDER_OPENAI_API_BASE=http://localhost:20199/v1
aider --model kr/claude-sonnet-4.5
```

#### Continue.dev Setup
```json
{
  "models": [
    {
      "title": "dntproxy - Claude Sonnet",
      "provider": "openai",
      "model": "kr/claude-sonnet-4.5",
      "apiBase": "http://localhost:20199/v1",
      "apiKey": "dummy-key"
    }
  ]
}
```

### 3. Document Limitations
- **Cursor**: May require workarounds or may not support custom proxies
- **GitHub Copilot**: Cannot be redirected to custom proxy

### 4. Consider Environment Variable Standards
Support common environment variables:
- `OPENAI_API_BASE` / `OPENAI_BASE_URL`
- `OPENAI_API_KEY`
- `ANTHROPIC_API_KEY`

This allows tools that respect these variables to work with dntproxy without additional configuration.

---

## Next Steps

1. **Test Integration**: Verify dntproxy works with Aider and Continue.dev
2. **Create Setup Guides**: Write detailed integration docs for each tool
3. **Add Examples**: Provide working config examples in dntproxy docs
4. **Environment Variable Support**: Ensure dntproxy respects standard env vars
5. **Auto-Configuration**: Consider providing setup scripts for common tools
