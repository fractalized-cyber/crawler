#!/bin/bash

# Basic usage examples for the web crawler

echo "Web Crawler - Basic Usage Examples"
echo "=================================="

# Build the crawler if it doesn't exist
if [ ! -f "../crawler" ]; then
    echo "Building crawler..."
    cd .. && make build && cd examples
fi

echo ""
echo "1. Basic crawling:"
echo "   ./crawler [url]"
echo ""

echo "2. Crawl with custom depth:"
echo "   ./crawler -depth 2 [url]"
echo ""

echo "3. Crawl with custom headers:"
echo "   ./crawler -H 'User-Agent: MyBot' [url]"
echo ""

echo "4. Crawl with more retries:"
echo "   ./crawler -retries 5 [url]"
echo ""

echo "5. Using -u flag:"
echo "   ./crawler -u [url]"
echo ""

echo "6. Save to custom directory:"
echo "   ./crawler -u [url] ./my-output"
echo ""

echo "7. Combined example:"
echo "   ./crawler -u [url] -depth 2 -H 'User-Agent: Mozilla/5.0' -retries 3 ./output"
echo ""

echo "Note: Replace '[url]' with your target URL"
echo "Output will be saved to './responses' by default" 