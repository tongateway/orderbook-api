# Open4Dev API

REST API for working with TON blockchain, orders, coins, and vaults.

## Description

Open4Dev API provides HTTP API for:
- Managing coins
- Working with orders
- Managing vaults
- Authentication via API keys
- Rate limiting for abuse protection

## Technologies

- **Go 1.24.5** - main programming language
- **Gin** - web framework
- **GORM** - ORM for database operations
- **PostgreSQL** - database
- **Redis** - for rate limiting
- **Swagger** - API documentation

## Requirements

- Go 1.24.5 or higher
- PostgreSQL 16 or higher
- Redis 7 or higher

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd api
```

2. Install dependencies:
```bash
go mod download
```

3. Configure the application:
```bash
cp configs/example.config.yaml configs/config.yaml
# Edit configs/config.yaml with your settings
```

4. Run database migrations:
```bash
# Use goose or another migration tool
goose -dir migrations postgres "your-connection-string" up
```

5. Run the application:
```bash
go run cmd/main.go
```

## Configuration

Create a `configs/config.yaml` file based on `configs/example.config.yaml`:

```yaml
redis:
  host: localhost
  port: 6379
  password: ""
  db: 0

network:
  mainnet: false

api:
  host: localhost
  port: 8000
  rps: 1
  auth_required: true

database:
  type: postgres
  host: localhost
  port: "5432"
  user: postgres
  password: your-password
  dbname: your-database
  sslmode: disable
```

## Docker

### Running with Docker Compose

1. Set environment variables (optional):
```bash
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=your-password
export POSTGRES_DB=open4dev
export REDIS_PASSWORD=your-redis-password
```

2. Start services:
```bash
docker-compose up -d
```

### Local Development

For local development, use `docker-compose.local.yaml`:
```bash
docker-compose -f docker-compose.local.yaml up -d
```

## API Documentation

After starting the server, Swagger documentation is available at:
- `http://localhost:8000/api/v1/swagger/index.html`

## Project Structure

```
api/
├── cmd/                    # Application entry point
├── configs/                # Configuration files
├── docs/                   # Swagger documentation
├── internal/
│   ├── config/            # Application configuration
│   ├── database/          # Database models
│   ├── handler/           # HTTP handlers
│   ├── logger/            # Logging
│   ├── middleware/        # Middleware (auth, rate limit, etc.)
│   ├── repository/        # Database repositories
│   └── services/          # Application services
├── migrations/            # Database migrations
└── readme                 # Old readme file
```

## API Endpoints

### Coins
- `GET /api/v1/coins` - List coins
- `GET /api/v1/coins/:id` - Get coin by ID

### Orders
- `GET /api/v1/orders` - List orders
- `GET /api/v1/orders/:id` - Get order by ID

### Vaults
- `GET /api/v1/vaults` - List vaults
- `GET /api/v1/vaults/:id` - Get vault by ID

## Authentication

The API uses API keys for authentication. Keys are stored in the database in the `api_keys` table.

To use the API, add the header:
```
Authorization: Bearer <your-api-key>
```

## Rate Limiting

The API limits the number of requests per second (RPS). The limit is configured in the configuration file via the `api.rps` parameter.

Rate limiting works based on the API key if provided, or by IP address if no key is provided.

## Migrations

Database migrations are located in the `migrations/` directory and use the goose format.

To apply migrations:
```bash
goose -dir migrations postgres "connection-string" up
```

To rollback migrations:
```bash
goose -dir migrations postgres "connection-string" down
```

## Development

### Running in Development Mode

```bash
go run cmd/main.go
```

### Generating Swagger Documentation

```bash
swag init -g cmd/main.go
```

## License

[Specify license]

## Contacts

[Specify contacts]
