require('dotenv').config();

module.exports = {
  port: parseInt(process.env.PORT, 10) || 3000,
  aws: {
    region: process.env.AWS_REGION || 'us-east-1',
    dynamoEndpoint: process.env.DYNAMO_ENDPOINT || undefined,
  },
  tables: {
    findings: process.env.DYNAMODB_FINDINGS_TABLE || 'ShadowAI_Findings',
    registry: process.env.DYNAMODB_REGISTRY_TABLE || 'ShadowAI_Registry',
    allowlist: process.env.DYNAMODB_ALLOWLIST_TABLE || 'ShadowAI_Allowlist',
    tenants: process.env.DYNAMODB_TENANTS_TABLE || 'ShadowAI_Tenants',
  },
  alerting: {
    slackWebhookUrl: process.env.SLACK_WEBHOOK_URL || '',
  },
  env: process.env.NODE_ENV || 'development',
};
