#!/bin/bash

echo "Initializing S3 buckets..."

sleep 5

export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1

echo "Creating S3 bucket: smartheart-files"
awslocal s3 mb s3://smartheart-files

echo "Setting bucket policy for public read access..."
cat > /tmp/bucket-policy.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PublicReadGetObject",
      "Effect": "Allow",
      "Principal": "*",
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::smartheart-files/*"
    }
  ]
}
EOF

awslocal s3api put-bucket-policy --bucket smartheart-files --policy file:///tmp/bucket-policy.json

echo "Setting CORS configuration..."
cat > /tmp/cors-config.json << 'EOF'
{
  "CORSRules": [
    {
      "AllowedOrigins": ["*"],
      "AllowedHeaders": ["*"],
      "AllowedMethods": ["GET", "PUT", "POST", "DELETE", "HEAD"],
      "MaxAgeSeconds": 3000
    }
  ]
}
EOF

awslocal s3api put-bucket-cors --bucket smartheart-files --cors-configuration file:///tmp/cors-config.json

echo "S3 buckets created:"
awslocal s3 ls

echo "S3 initialization completed!"
