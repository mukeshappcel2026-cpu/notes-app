const https = require('https');
const url = require('url');
const config = require('../config');

async function sendAlert(tenantId, finding) {
  if (!config.alerting.slackWebhookUrl) return;

  const riskEmoji = {
    high: ':red_circle:',
    medium: ':large_orange_circle:',
    low: ':white_circle:',
  };

  const payload = {
    blocks: [
      {
        type: 'header',
        text: {
          type: 'plain_text',
          text: `Shadow AI Detected: ${finding.providerName}`,
        },
      },
      {
        type: 'section',
        fields: [
          { type: 'mrkdwn', text: `*Risk Tier:*\n${riskEmoji[finding.riskTier] || ''} ${finding.riskTier}` },
          { type: 'mrkdwn', text: `*Category:*\n${finding.category}` },
          { type: 'mrkdwn', text: `*Source IP:*\n\`${finding.sourceIp}\`` },
          { type: 'mrkdwn', text: `*Detection Method:*\n${finding.detectionMethod}` },
          { type: 'mrkdwn', text: `*Domain/SNI:*\n\`${finding.queryName || finding.sniValue || 'N/A'}\`` },
          { type: 'mrkdwn', text: `*VPC:*\n${finding.sourceVpcId || 'N/A'}` },
        ],
      },
      {
        type: 'context',
        elements: [
          { type: 'mrkdwn', text: `Finding ID: \`${finding.findingId}\` | Detected: ${finding.detectedAt}` },
        ],
      },
    ],
  };

  await postJson(config.alerting.slackWebhookUrl, payload);
}

function postJson(targetUrl, data) {
  return new Promise((resolve, reject) => {
    const parsed = new URL(targetUrl);
    const body = JSON.stringify(data);

    const req = https.request({
      hostname: parsed.hostname,
      port: parsed.port || 443,
      path: parsed.pathname + parsed.search,
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(body),
      },
    }, (res) => {
      let responseBody = '';
      res.on('data', (chunk) => { responseBody += chunk; });
      res.on('end', () => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(responseBody);
        } else {
          reject(new Error(`Slack webhook returned ${res.statusCode}: ${responseBody}`));
        }
      });
    });

    req.on('error', reject);
    req.write(body);
    req.end();
  });
}

module.exports = { sendAlert };
