# Discord SSO Role System

A Go application that implements Discord role assignment based on Microsoft OAuth authentication. Users can verify their employee status through Microsoft login and automatically receive a Discord role.

## Features

- Microsoft OAuth integration for employee verification
- Discord bot with slash command `/verify-employee`
- In-memory verification code storage with TTL
- Web interface for code verification
- Docker support for easy deployment
- Configurable via environment variables

## Architecture

The application consists of:
- **Web Server**: Handles OAuth flow and verification (Gin framework)
- **Discord Bot**: Manages slash commands and role assignment (discordgo)
- **In-Memory Store**: Temporary storage for verification codes with expiration

## Prerequisites

1. Microsoft Azure App Registration
2. Discord Bot Token and Application
3. Discord Server with appropriate permissions
4. Go 1.21+ (for local development)
5. Docker (optional)

## Setup

### 1. Microsoft Azure Configuration

1. Create an App Registration in Azure Portal
2. Add redirect URI: `http://localhost:8080/auth/callback`
3. Create a client secret
4. Note down:
   - Application (client) ID
   - Directory (tenant) ID
   - Client Secret

### 2. Discord Configuration

1. Create a Discord Application at https://discord.com/developers/applications
2. Create a Bot and get the token
3. Invite the bot to your server with permissions:
   - `Manage Roles`
   - `Use Slash Commands`
4. Create or identify the role to assign
5. Note down:
   - Bot Token
   - Guild (Server) ID
   - Role ID

### 3. Environment Configuration

Copy `.env.example` to `.env` and fill in your values:

```bash
cp .env.example .env
```

Edit `.env` with your configuration:

```env
# Microsoft OAuth Configuration
MICROSOFT_CLIENT_ID=your-microsoft-client-id
MICROSOFT_CLIENT_SECRET=your-microsoft-client-secret
MICROSOFT_REDIRECT_URL=http://localhost:8080/auth/callback
MICROSOFT_TENANT_ID=your-tenant-id

# Discord Configuration
DISCORD_TOKEN=your-discord-bot-token
DISCORD_GUILD_ID=your-discord-guild-id
DISCORD_ROLE_ID=your-employee-role-id

# Server Configuration
PORT=8080
BASE_URL=http://localhost:8080
VERIFICATION_TTL=15
```

## Running the Application

### Using Docker Compose (Recommended)

```bash
docker-compose up --build
```

### Using Docker

```bash
# Build the image
docker build -t discord-sso-role .

# Run the container
docker run -p 8080:8080 --env-file .env discord-sso-role
```

### Local Development

```bash
# Install dependencies
go mod download

# Run the application
go run main.go
```

## Usage

1. **Discord User**: Use `/verify-employee` command in Discord
2. **OAuth Flow**: Click the provided link to authenticate with Microsoft
3. **Verification Code**: After successful authentication, you'll receive a verification code
4. **Complete Verification**: Enter the code on the verification page
5. **Role Assignment**: The bot will automatically assign the employee role

## API Endpoints

- `GET /` - Home page with verification form
- `GET /auth/start?state={discord_id}` - Start OAuth flow
- `GET /auth/callback` - OAuth callback
- `POST /verify` - Submit verification code
- `GET /health` - Health check endpoint

## Configuration Options

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| MICROSOFT_CLIENT_ID | Azure App Client ID | Required |
| MICROSOFT_CLIENT_SECRET | Azure App Client Secret | Required |
| MICROSOFT_REDIRECT_URL | OAuth Redirect URL | http://localhost:8080/auth/callback |
| MICROSOFT_TENANT_ID | Azure Tenant ID | Required |
| DISCORD_TOKEN | Discord Bot Token | Required |
| DISCORD_GUILD_ID | Discord Server ID | Required |
| DISCORD_ROLE_ID | Role ID to assign | Required |
| PORT | Server port | 8080 |
| BASE_URL | Application base URL | http://localhost:8080 |
| VERIFICATION_TTL | Code expiration in minutes | 15 |
| GIN_MODE | Gin framework mode | debug |

## Security Considerations

1. **Email Domain Validation**: The application validates email domains (currently set to @microsoft.com and @contoso.com). Modify in `handlers/discord.go` as needed.
2. **Verification Code TTL**: Codes expire after the configured TTL (default 15 minutes)
3. **One Code Per User**: Each Discord user can only have one active verification code
4. **HTTPS**: Use HTTPS in production by updating BASE_URL and MICROSOFT_REDIRECT_URL

## Troubleshooting

### Bot doesn't respond to commands
- Ensure the bot has proper permissions in the Discord server
- Check that the bot is online (green status)
- Verify the DISCORD_GUILD_ID is correct

### OAuth redirect fails
- Verify the redirect URI in Azure matches exactly
- Check MICROSOFT_REDIRECT_URL in environment

### Role assignment fails
- Ensure the bot's role is higher than the role being assigned
- Verify the DISCORD_ROLE_ID is correct
- Check bot has "Manage Roles" permission

## Development

### Project Structure
```
.
├── main.go              # Application entry point
├── handlers/            # HTTP and Discord handlers
│   ├── discord.go       # Discord bot logic
│   ├── oauth.go         # OAuth flow handlers
│   └── web.go           # Web interface handlers
├── models/              # Data models
│   ├── config.go        # Configuration model
│   └── verification.go  # Verification code store
├── utils/               # Utilities
│   └── logger.go        # Logger configuration
├── templates/           # HTML templates
│   ├── index.html       # Home page
│   ├── success.html     # OAuth success page
│   └── error.html       # Error page
├── Dockerfile           # Docker configuration
├── docker-compose.yml   # Docker Compose configuration
└── go.mod               # Go module file
```

### Adding Email Domain Validation

Edit `handlers/discord.go` in the `VerifyUser` method to add your allowed domains:

```go
// Check if email is from allowed domain
if !strings.HasSuffix(vc.Email, "@yourdomain.com") {
    h.store.Delete(code)
    return fmt.Errorf("email domain not allowed")
}
```

## License

This project is provided as-is for demonstration purposes.