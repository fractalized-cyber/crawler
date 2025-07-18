# Web Crawler

A Go application that crawls websites using a headless Chrome browser, capturing both page content and resources with enhanced URL resolution and retry logic.

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)]()

## Features

- **Comprehensive crawling**: Crawls both links and resources (JS, CSS, images)
- **Enhanced URL resolution**: Tries both current directory and root directory for relative paths
- **Retry logic**: Handles connection issues with configurable retry attempts
- **Resource fetching**: Downloads and saves JavaScript, CSS, and image files
- **Metadata tracking**: Tracks source information for discovered links
- **Custom headers**: Supports custom HTTP headers for authentication or user-agent spoofing
- **Configurable depth**: Control crawling depth to avoid infinite loops
- **Robust error handling**: Better handling of network issues and timeouts

## Quick Start

```bash
# Clone the repository
git clone https://github.com/fractalized-cyber/crawler.git
cd crawler

# Build the crawler
make build

# Run a basic crawl
./crawler [url]
```

## Prerequisites

- Go 1.21 or later
- Chrome/Chromium browser installed on your system

## Installation

1. Clone or download this repository
2. Install dependencies:
   ```bash
   make deps
   ```
3. Build the crawler:
   ```bash
   make build
   ```

## Usage

```bash
go run main.go [flags] <url> [output_directory]
./crawler [flags] <url> [output_directory]
./crawler -u <url> [flags] [output_directory]
```

### Flags

- `-u url` - Target URL to crawl
- `-H header` - Custom header (can be used multiple times)
- `-depth N` - Maximum crawl depth (default: 5)
- `-retries N` - Maximum retry attempts for failed connections (default: 3)

### Examples

```bash
# Basic crawling
go run main.go [url]
./crawler [url]
./crawler -u [url]

# Crawl with custom depth and output directory
./crawler -u [url] -depth 3 ./output

# Crawl with custom headers
./crawler -H 'User-Agent: MyBot' -depth 2 [url]

# More retries for unstable connections
./crawler -retries 5 [url]

# Browser-like headers to avoid detection
./crawler -H "User-Agent: Mozilla/5.0" -H "Accept: text/html,application/xhtml+xml" [url]
```

See `examples/basic-usage.sh` for more usage examples.

## Output

The application creates the following files in the output directory:

- `final_page.html` - The final HTML content of the initial page
- Individual response files named `1_<url>.html`, `2_<url>.js`, etc. with appropriate extensions

Each response file contains:
- URL
- Response body (HTML, JavaScript, CSS, etc.)
- MIME type
- File size

## How it Works

1. **Initial page load**: Loads the target URL with retry logic
2. **Link discovery**: Extracts all `<a href>` links with metadata
3. **Resource discovery**: Finds JavaScript, CSS, and image files
4. **Enhanced resolution**: Resolves relative URLs against both current and root directories
5. **Crawling**: Visits discovered links up to the specified depth
6. **Resource fetching**: Downloads and saves resource files
7. **Output**: Saves all content with appropriate file extensions

## Advanced Features

### Enhanced URL Resolution
The crawler tries multiple base paths for relative URLs:
- **Primary**: Resolves against current page URL
- **Fallback**: Also tries site root directory
- **Debug output**: Shows when multiple URLs are resolved

### Retry Logic
- **Configurable retries**: Default 3 attempts, customizable via `-retries` flag
- **Progressive delays**: Waits between retry attempts
- **Connection handling**: Better handling of `ERR_CONNECTION_CLOSED` errors

### Resource Fetching
- **JavaScript files**: Downloads and saves `.js` files
- **CSS files**: Downloads and saves `.css` files  
- **Images**: Downloads and saves image files
- **MIME detection**: Automatically detects file types

## Development

### Building

```bash
# Basic build
make build

# Development build with race detection
make dev

# Build for specific platforms
make build-linux
make build-windows
make build-darwin

# Build for all platforms
make build-all
```

### Code Quality

```bash
# Format code
make fmt

# Lint code (requires golangci-lint)
make lint

# Run tests
make test
```

## Troubleshooting

### Connection Issues
If you encounter `ERR_CONNECTION_CLOSED` errors:

```bash
# Try with more retries
./crawler -retries 5 [url]

# Use browser-like headers
./crawler -H "User-Agent: Mozilla/5.0" [url]
```

### Common Issues
1. **Chrome not found**: Ensure Chrome/Chromium is installed
2. **Permission errors**: Check write permissions for output directory
3. **Network issues**: Try increasing retry count or using custom headers
4. **Rate limiting**: Add delays or use different user agents

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Security Note

This tool captures all content from the specified website. Be careful when using it on sites that handle sensitive information, as all response data will be saved to disk.

## Disclaimer

This tool is for educational and legitimate web scraping purposes only. Always respect robots.txt files and website terms of service. The authors are not responsible for any misuse of this software. 
