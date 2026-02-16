# Notes API

A RESTful API for managing notes, built with Express.js and DynamoDB, deployed to AWS with GitHub Actions CI/CD.

## 🚀 Features

- ✅ Create, Read, Update, Delete (CRUD) operations for notes
- ✅ User-based note organization
- ✅ DynamoDB backend for scalable storage
- ✅ CORS enabled for frontend integration
- ✅ Comprehensive test coverage (unit, integration, E2E)
- ✅ Automated CI/CD with GitHub Actions
- ✅ Health check endpoint
- ✅ Input validation and error handling

## 📋 Prerequisites

- Node.js 18+
- AWS Account
- AWS CLI configured
- GitHub account

## 🛠️ Local Development

### Install Dependencies

```bash
npm install
```

### Run Locally

```bash
# Start server
npm start

# Development mode with auto-reload
npm run dev
```

The server will start on `http://localhost:3000`

### Environment Variables

```bash
PORT=3000                    # Server port (default: 3000)
AWS_REGION=us-east-1        # AWS region (default: us-east-1)
DYNAMODB_TABLE=Notes        # DynamoDB table name (default: Notes)
NODE_ENV=development        # Environment (development/production/test)
```

## 🧪 Testing

### Run All Tests

```bash
npm test
```

### Run Specific Test Suites

```bash
# Unit tests only
npm run test:unit

# Integration tests only
npm run test:integration

# E2E tests only
npm run test:e2e

# Watch mode
npm run test:watch
```

### Test Coverage

Tests are configured to maintain minimum coverage:
- Branches: 70%
- Functions: 80%
- Lines: 80%
- Statements: 80%

## 📡 API Endpoints

### Health Check

```http
GET /health
```

Response:
```json
{
  "status": "healthy",
  "timestamp": "2026-02-15T12:00:00.000Z",
  "service": "Notes API",
  "version": "1.0.0"
}
```

### Create Note

```http
POST /notes
Content-Type: application/json

{
  "userId": "user123",
  "title": "My Note",
  "content": "Note content here"
}
```

### Get All Notes for User

```http
GET /notes/:userId
```

### Get Single Note

```http
GET /notes/:userId/:noteId
```

### Update Note

```http
PUT /notes/:userId/:noteId
Content-Type: application/json

{
  "title": "Updated Title",
  "content": "Updated content"
}
```

### Delete Note

```http
DELETE /notes/:userId/:noteId
```

## 🔄 CI/CD Pipeline

### Continuous Integration (on Pull Request)

1. **Lint** - Code style checking
2. **Unit Tests** - Test individual functions
3. **Integration Tests** - Test database operations
4. **E2E Tests** - Test API endpoints
5. **Security Audit** - Check for vulnerabilities
6. **Build** - Create deployment package

### Continuous Deployment (on merge to main)

1. **Run Tests** - Ensure code quality
2. **Build Package** - Create deployment artifact
3. **Configure AWS** - Use OIDC for secure authentication
4. **Deploy to EC2** - Update running application
5. **Health Check** - Verify deployment success
6. **Smoke Tests** - Test live API
7. **Rollback** - Automatic rollback on failure

## 🔐 Security

- ✅ AWS OIDC authentication (no stored credentials)
- ✅ Input validation on all endpoints
- ✅ Automated security audits
- ✅ CORS configured for specific origins
- ✅ Environment-specific configurations

## 📊 GitHub Actions Workflows

### CI Workflow (.github/workflows/ci.yml)

Runs on every pull request:
- Lints code
- Runs all tests
- Checks security vulnerabilities
- Builds deployment package

### Deploy Workflow (.github/workflows/deploy.yml)

Runs on merge to main:
- Deploys to AWS EC2
- Runs health checks
- Performs smoke tests
- Automatic rollback on failure

## 🏗️ Project Structure

```
notes-app/
├── .github/
│   └── workflows/
│       ├── ci.yml              # CI pipeline
│       └── deploy.yml          # Deployment pipeline
├── src/
│   └── server.js               # Main application
├── tests/
│   ├── unit/                   # Unit tests
│   ├── integration/            # Integration tests
│   └── e2e/                    # End-to-end tests
├── package.json
└── README.md
```

## 🔧 Configuration

### GitHub Secrets (Required)

Add these in GitHub: Settings → Secrets and variables → Actions

```
AWS_ROLE_ARN=arn:aws:iam::ACCOUNT_ID:role/GitHubActions-App-Deploy
```

### GitHub Variables (Required)

```
AWS_REGION=us-east-1
DEPLOYMENT_BUCKET=your-deployment-bucket-name
```

## 🚀 Deployment

### Prerequisites

1. AWS infrastructure deployed (see notes-infrastructure repo)
2. GitHub Actions secrets configured
3. EC2 instance has SSM agent enabled

### Deploy

1. Push to main branch (automatic deployment)
2. Or manually trigger: Actions → Deploy to AWS → Run workflow

### Rollback

If deployment fails, the workflow automatically rolls back to the previous version.

Manual rollback:
```bash
# SSH into EC2
ssh ec2-user@YOUR-EC2-IP

# Rollback
cd /home/ec2-user
sudo systemctl stop notes-app
rm -rf notes-app
mv notes-app-old notes-app
sudo systemctl start notes-app
```

## 📈 Monitoring

### Check Application Status

```bash
# Via SSM
aws ssm start-session --target INSTANCE_ID

# Check service status
sudo systemctl status notes-app

# View logs
sudo journalctl -u notes-app -f
```

### Health Check

```bash
curl http://YOUR-ALB-URL/health
```

## 🐛 Troubleshooting

### Tests Failing

```bash
# Clear cache and reinstall
rm -rf node_modules package-lock.json
npm install

# Run tests with verbose output
npm test -- --verbose
```

### Deployment Failing

1. Check GitHub Actions logs
2. Verify AWS credentials and permissions
3. Check EC2 instance is running
4. Verify SSM agent is active

### Application Not Responding

```bash
# SSH into EC2
aws ssm start-session --target INSTANCE_ID

# Check application logs
sudo journalctl -u notes-app -n 100

# Restart application
sudo systemctl restart notes-app
```

## 📝 Contributing

1. Create a feature branch
2. Make your changes
3. Write/update tests
4. Submit a pull request
5. Wait for CI to pass
6. Get approval from maintainers

## 📄 License

MIT

## 🤝 Related Repositories

- [notes-infrastructure](https://github.com/mukeshappcel2026-cpu/notes-infrastructure) - Terraform infrastructure as code
- Frontend - (to be added)

## 📞 Support

For issues and questions:
- Create an issue in GitHub
- Check existing documentation
- Review CI/CD logs for errors
