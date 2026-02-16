require('dotenv').config();

const config = {
  port: parseInt(process.env.PORT, 10) || 3000,
  region: process.env.AWS_REGION || 'us-east-1',
  tableName: process.env.DYNAMODB_TABLE || 'Notes',
  nodeEnv: process.env.NODE_ENV || 'development',
  allowedOrigin: process.env.ALLOWED_ORIGIN || '*',
  // DynamoDB Local endpoint for docker-compose dev
  // When null, the AWS SDK uses the default AWS endpoint
  dynamoEndpoint: process.env.DYNAMO_ENDPOINT || null,
  // Google OAuth 2.0 Client ID for authentication
  googleClientId: process.env.GOOGLE_CLIENT_ID || null,
};

module.exports = config;
