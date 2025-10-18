#!/bin/bash

echo "Checking PostgreSQL connection..."
if nc -z localhost 5432; then
    echo " PostgreSQL is accessible on port 5432"
else
    echo " PostgreSQL is NOT accessible on port 5432"
fi

echo "Checking Redis connection..."
if nc -z localhost 6379; then
    echo " Redis is accessible on port 6379"
else
    echo " Redis is NOT accessible on port 6379"
fi

echo "Checking Docker containers..."
docker-compose ps
