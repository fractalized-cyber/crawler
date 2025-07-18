package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"golang.org/x/net/html"
)

type ResponseData struct {
	URL      string `json:"url"`
	Body     string `json:"body"`
	MimeType string `json:"mime_type"`
}

// AbsoluteURL resolves a relative path against the response URL
func (r *ResponseData) AbsoluteURL(path string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	resolvedURLs := resolveURLWithFallback(path, r.URL)
	if len(resolvedURLs) > 0 {
		return resolvedURLs[0] // Return the primary resolution
	}
	return path
}

// Request represents a navigation request for the crawler
type Request struct {
	Method         string              `json:"method,omitempty"`
	URL            string              `json:"url,omitempty"`
	Body           string              `json:"body,omitempty"`
	Depth          int                 `json:"depth,omitempty"`
	SkipValidation bool                `json:"-"`
	Headers        map[string]string   `json:"headers,omitempty"`
	Tag            string              `json:"tag,omitempty"`
	Attribute      string              `json:"attribute,omitempty"`
	RootHostname   string              `json:"-"`
	Source         string              `json:"source,omitempty"`
	CustomFields   map[string][]string `json:"-"`
	Raw            string              `json:"raw,omitempty"`
}

// RequestURL returns the request URL for the navigation
func (r *Request) RequestURL() string {
	switch r.Method {
	case "GET":
		return r.URL
	case "POST":
		builder := &strings.Builder{}
		builder.WriteString(r.URL)
		builder.WriteString(":")
		builder.WriteString(r.Body)
		return builder.String()
	}
	return ""
}

// NewRequestFromURL creates a new request from a URL
func NewRequestFromURL(urlStr, rootHostname string, depth int) *Request {
	return &Request{
		Method:       http.MethodGet,
		URL:          urlStr,
		RootHostname: rootHostname,
		Depth:        depth,
	}
}

// NewRequestFromResponse creates a new request from a response
func NewRequestFromResponse(path, source, tag, attribute string, resp *ResponseData, rootHostname string, depth int) *Request {
	requestURL := resp.AbsoluteURL(path)
	return &Request{
		Method:       http.MethodGet,
		URL:          requestURL,
		RootHostname: rootHostname,
		Depth:        depth,
		Source:       source,
		Attribute:    attribute,
		Tag:          tag,
	}
}

type NetworkCapture struct {
	TargetHost    string
	Responses     []ResponseData
	OutputDir     string
	CustomHeaders map[string]string
	VisitedURLs   map[string]bool
	MaxDepth      int
}

// LinkInfo represents a link with metadata
type LinkInfo struct {
	URL       string
	Tag       string
	Attribute string
	Text      string
}

func main() {
	// Define URL flag
	var targetURL string
	flag.StringVar(&targetURL, "u", "", "Target URL to crawl")

	// Define custom headers flag
	var headers []string
	flag.Var((*stringSlice)(&headers), "H", "Custom header (can be used multiple times, e.g., -H 'User-Agent: MyBot' -H 'Accept: application/json')")

	// Define crawl depth flag
	var crawlDepth int
	flag.IntVar(&crawlDepth, "depth", 5, "Maximum crawl depth (default: 5)")

	// Define retry flag
	var maxRetries int
	flag.IntVar(&maxRetries, "retries", 3, "Maximum number of retry attempts for failed connections (default: 3)")

	// Parse flags
	flag.Parse()

	// Get remaining arguments after flags
	args := flag.Args()

	// Check if URL is provided via flag or argument
	if targetURL == "" {
		if len(args) < 1 {
			fmt.Println("Usage: go run main.go [flags] <url> [output_directory]")
			fmt.Println("       ./crawler [flags] <url> [output_directory]")
			fmt.Println("       ./crawler -u <url> [flags] [output_directory]")
			fmt.Println("Flags:")
			fmt.Println("  -u url              Target URL to crawl")
			fmt.Println("  -H header           Custom header (can be used multiple times)")
			fmt.Println("  -depth N            Maximum crawl depth (default: 5)")
			fmt.Println("  -retries N          Maximum retry attempts for failed connections (default: 3)")
			fmt.Println("")
			fmt.Println("Examples:")
			fmt.Println("  go run main.go [url]")
			fmt.Println("  ./crawler [url]")
			fmt.Println("  ./crawler -u [url]")
			fmt.Println("  ./crawler -u [url] -depth 3 ./output")
			fmt.Println("  ./crawler -H 'User-Agent: MyBot' -depth 2 [url]")
			fmt.Println("  ./crawler -retries 5 [url]")
			os.Exit(1)
		}
		targetURL = args[0]
		args = args[1:] // Remove URL from args
	}

	outputDir := "./responses"
	if len(args) > 0 {
		outputDir = args[0]
		// Check if the output directory argument looks like a flag
		if strings.HasPrefix(outputDir, "-") {
			fmt.Printf("Error: '%s' looks like a flag. Did you mean to specify an output directory?\n", outputDir)
			fmt.Println("Usage: go run main.go [flags] <url> [output_directory]")
			fmt.Println("       ./crawler [flags] <url> [output_directory]")
			fmt.Println("       ./crawler -u <url> [flags] [output_directory]")
			os.Exit(1)
		}
	}

	// Parse custom headers
	customHeaders := make(map[string]string)
	for _, header := range headers {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			customHeaders[key] = value
		} else {
			log.Printf("Warning: Invalid header format '%s', expected 'Key: Value'", header)
		}
	}

	fmt.Printf("Debug: Parsed arguments - URL: %s, OutputDir: %s\n", targetURL, outputDir)
	fmt.Printf("Debug: Custom headers: %v\n", customHeaders)

	// Parse the target URL to extract host
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		log.Fatal("Invalid URL:", err)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatal("Failed to create output directory:", err)
	}

	capture := &NetworkCapture{
		TargetHost:    normalizeHost(parsedURL.Host),
		Responses:     make([]ResponseData, 0),
		OutputDir:     outputDir,
		CustomHeaders: customHeaders,
		VisitedURLs:   make(map[string]bool),
		MaxDepth:      crawlDepth, // Use the parsed depth
	}

	fmt.Printf("Starting crawler for: %s\n", targetURL)
	fmt.Printf("Output directory: %s\n", outputDir)
	if len(customHeaders) > 0 {
		fmt.Printf("Custom headers: %d configured\n", len(customHeaders))
	}

	// Create Chrome context
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	// Set timeout
	ctx, cancel = context.WithTimeout(ctx, 300*time.Second) // Increased to 5 minutes
	defer cancel()

	// Enable network events
	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		log.Fatal("Failed to enable network:", err)
	}

	// Set custom headers if provided
	if len(customHeaders) > 0 {
		headers := make(map[string]interface{})
		for key, value := range customHeaders {
			headers[key] = value
		}
		if err := chromedp.Run(ctx, network.SetExtraHTTPHeaders(headers)); err != nil {
			log.Printf("Warning: Failed to set custom headers: %v", err)
		}
	}

	// Navigate to the page with retry logic
	fmt.Printf("Loading initial page...\n")
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("   ⏳ Retry attempt %d/%d...\n", attempt, maxRetries)
			time.Sleep(2 * time.Second) // Wait before retry
		}

		err := chromedp.Run(ctx, chromedp.Navigate(targetURL))
		if err == nil {
			break // Success
		}

		if attempt == maxRetries {
			log.Fatal("Failed to navigate after all retry attempts")
		}
	}

	// Wait for page to load completely
	time.Sleep(2 * time.Second)

	// Wait a bit more for any additional requests
	time.Sleep(1 * time.Second)

	// Capture the final HTML content of the page
	fmt.Printf("Capturing initial page content...\n")
	var finalHTML string
	if err := chromedp.Run(ctx, chromedp.OuterHTML("html", &finalHTML)); err != nil {
		log.Printf("Warning: Could not capture final page HTML: %v", err)
	} else {
		if len(finalHTML) > 0 {
			// Save the final HTML as a separate file
			finalHTMLFile := filepath.Join(outputDir, "final_page.html")
			if err := os.WriteFile(finalHTMLFile, []byte(finalHTML), 0644); err != nil {
				log.Printf("Failed to write final HTML file: %v", err)
			} else {
				fmt.Printf("   ✅ Saved initial page (%d bytes)\n", len(finalHTML))
			}
		}
	}

	// Start crawling process
	fmt.Printf("Starting crawl process (max depth: %d)...\n", capture.MaxDepth)

	// Initialize crawl queue with the initial URL
	crawlQueue := []*Request{NewRequestFromURL(targetURL, capture.TargetHost, 0)}
	processedURLs := make(map[string]bool)

	for len(crawlQueue) > 0 {
		// Get next job from queue
		job := crawlQueue[0]
		crawlQueue = crawlQueue[1:]

		// Skip if already processed
		if processedURLs[job.URL] {
			continue
		}

		processedURLs[job.URL] = true
		fmt.Printf("\nCrawling [%d/%d]: %s\n", job.Depth+1, capture.MaxDepth+1, job.URL)
		if job.Source != "" {
			fmt.Printf("   From: %s\n", job.Source)
		}

		// Navigate to the URL with retry logic
		var navigateErr error
		for attempt := 1; attempt <= maxRetries; attempt++ {
			if attempt > 1 {
				fmt.Printf("   Retry %d/%d...\n", attempt, maxRetries)
				time.Sleep(1 * time.Second)
			}

			navigateErr = chromedp.Run(ctx, chromedp.Navigate(job.URL))
			if navigateErr == nil {
				break // Success
			}
		}

		if navigateErr != nil {
			fmt.Printf("   Failed to load: %v\n", navigateErr)
			continue
		}

		// Wait for page to load
		time.Sleep(1 * time.Second)

		// Get the page HTML
		var pageHTML string
		if err := chromedp.Run(ctx, chromedp.OuterHTML("html", &pageHTML)); err != nil {
			fmt.Printf("   Failed to get HTML: %v\n", err)
			continue
		}

		// Save the page HTML as a response
		if len(pageHTML) > 0 {
			responseData := ResponseData{
				URL:      job.URL,
				Body:     pageHTML,
				MimeType: "text/html",
			}
			capture.Responses = append(capture.Responses, responseData)
			fmt.Printf("   Page saved (%d bytes)\n", len(pageHTML))

			// Extract and save additional resources
			resources := extractResources(pageHTML, job.URL)
			if len(resources) > 0 {
				fmt.Printf("   Found %d resources\n", len(resources))

				// Fetch and save resources that are on the same domain
				savedResources := 0
				for _, resource := range resources {
					if isSameDomain(capture.TargetHost, resource) {
						resourceBody, resourceMimeType := fetchResource(ctx, resource)

						// Create a resource response entry
						resourceData := ResponseData{
							URL:      resource,
							Body:     resourceBody,
							MimeType: resourceMimeType,
						}
						capture.Responses = append(capture.Responses, resourceData)
						savedResources++
					}
				}
				if savedResources > 0 {
					fmt.Printf("   Saved %d resources\n", savedResources)
				}
			}
		}

		// Extract links from the page with metadata
		links := extractLinksWithMetadata(pageHTML, job.URL)
		if len(links) > 0 {
			fmt.Printf("   Found %d links\n", len(links))

			// Add new URLs to crawl queue if within depth limit
			if job.Depth < capture.MaxDepth {
				queuedCount := 0
				for _, linkInfo := range links {
					if isSameDomain(capture.TargetHost, linkInfo.URL) && !processedURLs[linkInfo.URL] {
						// Check if this URL is already in the queue
						alreadyQueued := false
						for _, queuedJob := range crawlQueue {
							if queuedJob.URL == linkInfo.URL {
								alreadyQueued = true
								break
							}
						}

						if !alreadyQueued {
							newRequest := NewRequestFromResponse(linkInfo.URL, job.URL, linkInfo.Tag, linkInfo.Attribute, &ResponseData{URL: job.URL}, capture.TargetHost, job.Depth+1)
							crawlQueue = append(crawlQueue, newRequest)
							queuedCount++
						}
					}
				}
				if queuedCount > 0 {
					fmt.Printf("   Queued %d new URLs for crawling\n", queuedCount)
				}
			}
		}

		// Wait a bit before next crawl to be respectful
		time.Sleep(500 * time.Millisecond)
	}

	// Save all captured responses
	capture.SaveResponses()

	fmt.Printf("\nCrawl complete! Saved %d responses to %s\n", len(capture.Responses), outputDir)
}

// stringSlice type for flag parsing
type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func (nc *NetworkCapture) SaveResponses() {
	fmt.Printf("\nSaving responses...\n")

	// Save individual response content files
	savedCount := 0
	for i, response := range nc.Responses {
		// Create a safe filename
		safeURL := strings.ReplaceAll(response.URL, "://", "_")
		safeURL = strings.ReplaceAll(safeURL, "/", "_")
		safeURL = strings.ReplaceAll(safeURL, "?", "_")
		safeURL = strings.ReplaceAll(safeURL, "&", "_")
		safeURL = strings.ReplaceAll(safeURL, "=", "_")

		// Limit filename length
		if len(safeURL) > 100 {
			safeURL = safeURL[:100]
		}

		// Save response content as separate file with appropriate extension
		if len(response.Body) > 0 {
			extension := getFileExtension(response.MimeType, []byte(response.Body))
			contentFilename := fmt.Sprintf("%d_%s%s", i+1, safeURL, extension)
			contentFilepath := filepath.Join(nc.OutputDir, contentFilename)

			if err := os.WriteFile(contentFilepath, []byte(response.Body), 0644); err != nil {
				log.Printf("Failed to write content file %d: %v", i+1, err)
			} else {
				savedCount++
			}
		}
	}

	fmt.Printf("   Saved %d files\n", savedCount)
}

func normalizeHost(host string) string {
	host = strings.ToLower(host)
	if strings.HasPrefix(host, "www.") {
		host = strings.TrimPrefix(host, "www.")
	}
	// Remove port if present
	if colon := strings.Index(host, ":"); colon != -1 {
		host = host[:colon]
	}
	return host
}

func isSameOrSubdomain(target, candidate string) bool {
	target = normalizeHost(target)
	candidate = normalizeHost(candidate)
	return candidate == target || strings.HasSuffix(candidate, "."+target)
}

// Helper function to extract links from HTML content
func extractLinks(htmlContent string, baseURL string) []string {
	var links []string
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return links
	}

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					link := attr.Val
					// Resolve relative URLs
					if !strings.HasPrefix(link, "http") {
						base, _ := url.Parse(baseURL)
						linkURL, _ := url.Parse(link)
						if linkURL != nil {
							link = base.ResolveReference(linkURL).String()
						}
					}
					links = append(links, link)
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(doc)
	return links
}

// Helper function to extract links with metadata from HTML content
func extractLinksWithMetadata(htmlContent string, baseURL string) []LinkInfo {
	var links []LinkInfo
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return links
	}

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			var linkInfo LinkInfo
			linkInfo.Tag = "a"
			linkInfo.Attribute = "href"

			// Extract link text
			var text strings.Builder
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					text.WriteString(c.Data)
				}
			}
			linkInfo.Text = strings.TrimSpace(text.String())

			for _, attr := range n.Attr {
				if attr.Key == "href" {
					link := attr.Val
					// Use enhanced URL resolution that tries both current directory and root
					resolvedURLs := resolveURLWithFallback(link, baseURL)
					if len(resolvedURLs) > 0 {
						linkInfo.URL = resolvedURLs[0] // Use the first (primary) resolution
						links = append(links, linkInfo)

						// If there are additional resolutions, create additional link entries
						for i := 1; i < len(resolvedURLs); i++ {
							additionalLink := linkInfo
							additionalLink.URL = resolvedURLs[i]
							links = append(links, additionalLink)
						}
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(doc)
	return links
}

// Helper function to check if URL is on the same domain
func isSameDomain(targetHost, urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return normalizeHost(parsedURL.Host) == targetHost
}

// Crawl function to visit URLs and extract more links
func (nc *NetworkCapture) crawlURL(ctx context.Context, job *Request) {
	if job.Depth > nc.MaxDepth {
		return
	}

	// Check if we've already visited this URL
	if nc.VisitedURLs[job.URL] {
		return
	}

	nc.VisitedURLs[job.URL] = true
	fmt.Printf("Crawling (depth %d): %s\n", job.Depth, job.URL)

	// Navigate to the URL
	if err := chromedp.Run(ctx, chromedp.Navigate(job.URL)); err != nil {
		log.Printf("Failed to navigate to %s: %v", job.URL, err)
		return
	}

	// Wait for page to load
	time.Sleep(2 * time.Second)

	// Get the page HTML
	var pageHTML string
	if err := chromedp.Run(ctx, chromedp.OuterHTML("html", &pageHTML)); err != nil {
		log.Printf("Failed to get HTML from %s: %v", job.URL, err)
		return
	}

	// Extract links from the page
	links := extractLinks(pageHTML, job.URL)
	fmt.Printf("  Found %d links on %s\n", len(links), job.URL)

	// Queue new URLs for crawling
	for _, link := range links {
		if isSameDomain(nc.TargetHost, link) {
			// Add to crawl queue for next iteration
			// For now, we'll process them in the main loop
			// In a more sophisticated version, you'd use a proper queue
		}
	}
}

// Helper function to extract additional resources from HTML
func extractResources(htmlContent string, baseURL string) []string {
	var resources []string
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return resources
	}

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script":
				for _, attr := range n.Attr {
					if attr.Key == "src" {
						resolvedURLs := resolveURLWithFallback(attr.Val, baseURL)
						resources = append(resources, resolvedURLs...)
						break
					}
				}
			case "link":
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						resolvedURLs := resolveURLWithFallback(attr.Val, baseURL)
						resources = append(resources, resolvedURLs...)
						break
					}
				}
			case "img":
				for _, attr := range n.Attr {
					if attr.Key == "src" {
						resolvedURLs := resolveURLWithFallback(attr.Val, baseURL)
						resources = append(resources, resolvedURLs...)
						break
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(doc)
	return resources
}

// Helper function to resolve relative URLs
func resolveURL(urlStr, baseURL string) string {
	if strings.HasPrefix(urlStr, "http") {
		return urlStr
	}
	base, _ := url.Parse(baseURL)
	linkURL, _ := url.Parse(urlStr)
	if linkURL != nil {
		return base.ResolveReference(linkURL).String()
	}
	return urlStr
}

// Enhanced URL resolution that tries multiple base paths
func resolveURLWithFallback(urlStr, baseURL string) []string {
	if strings.HasPrefix(urlStr, "http") {
		return []string{urlStr}
	}

	var resolvedURLs []string

	// Parse the base URL
	base, err := url.Parse(baseURL)
	if err != nil {
		return []string{urlStr}
	}

	// Parse the relative URL
	linkURL, err := url.Parse(urlStr)
	if err != nil {
		return []string{urlStr}
	}

	// Method 1: Resolve against current page URL (standard behavior)
	primaryURL := base.ResolveReference(linkURL).String()
	resolvedURLs = append(resolvedURLs, primaryURL)

	// Method 2: If it's a relative path (not starting with /), also try root directory
	if !strings.HasPrefix(urlStr, "/") {
		// Create root URL by removing path from base URL
		rootURL := *base
		rootURL.Path = "/"
		rootURL.RawPath = "/"

		// Resolve against root
		rootResolved := rootURL.ResolveReference(linkURL).String()
		if rootResolved != primaryURL {
			resolvedURLs = append(resolvedURLs, rootResolved)
		}
	}

	return resolvedURLs
}

// Helper function to determine MIME type from URL
func getMimeTypeFromURL(urlStr string) string {
	urlStr = strings.ToLower(urlStr)
	switch {
	case strings.HasSuffix(urlStr, ".js"):
		return "application/javascript"
	case strings.HasSuffix(urlStr, ".css"):
		return "text/css"
	case strings.HasSuffix(urlStr, ".png"):
		return "image/png"
	case strings.HasSuffix(urlStr, ".jpg") || strings.HasSuffix(urlStr, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(urlStr, ".gif"):
		return "image/gif"
	case strings.HasSuffix(urlStr, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(urlStr, ".json"):
		return "application/json"
	case strings.HasSuffix(urlStr, ".xml"):
		return "application/xml"
	case strings.HasSuffix(urlStr, ".pdf"):
		return "application/pdf"
	default:
		return "text/plain"
	}
}

// Helper function to determine file extension based on MIME type and content
func getFileExtension(mimeType string, body []byte) string {
	mimeType = strings.ToLower(mimeType)

	// Check MIME type first
	switch {
	case strings.Contains(mimeType, "html"):
		return ".html"
	case strings.Contains(mimeType, "json"):
		return ".json"
	case strings.Contains(mimeType, "xml"):
		return ".xml"
	case strings.Contains(mimeType, "javascript") || strings.Contains(mimeType, "js"):
		return ".js"
	case strings.Contains(mimeType, "css"):
		return ".css"
	case strings.Contains(mimeType, "png"):
		return ".png"
	case strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg"):
		return ".jpg"
	case strings.Contains(mimeType, "gif"):
		return ".gif"
	case strings.Contains(mimeType, "svg"):
		return ".svg"
	case strings.Contains(mimeType, "pdf"):
		return ".pdf"
	case strings.Contains(mimeType, "text/plain"):
		return ".txt"
	}

	// If MIME type doesn't help, try to detect from content
	bodyStr := strings.ToLower(string(body))
	if strings.Contains(bodyStr, "<html") || strings.Contains(bodyStr, "<!doctype") {
		return ".html"
	}
	if strings.HasPrefix(strings.TrimSpace(bodyStr), "{") || strings.HasPrefix(strings.TrimSpace(bodyStr), "[") {
		return ".json"
	}
	if strings.Contains(bodyStr, "<?xml") || strings.HasPrefix(strings.TrimSpace(bodyStr), "<") {
		return ".xml"
	}

	// Default to .txt for unknown types
	return ".txt"
}

// Helper function to fetch resource content
func fetchResource(ctx context.Context, resourceURL string) (string, string) {
	// Try to fetch the resource using Chrome DevTools Protocol
	var resourceBody string
	var resourceMimeType string

	// Create a timeout context for resource fetching
	resourceCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Use chromedp to fetch the resource
	err := chromedp.Run(resourceCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		// Enable network events if not already enabled
		if err := network.Enable().Do(ctx); err != nil {
			return err
		}

		// Navigate to the resource
		if err := chromedp.Run(ctx, chromedp.Navigate(resourceURL)); err != nil {
			return err
		}

		// Wait a bit for the resource to load
		time.Sleep(500 * time.Millisecond)

		// Get the page content
		if err := chromedp.Run(ctx, chromedp.OuterHTML("html", &resourceBody)); err != nil {
			// If it's not HTML, try to get the raw content
			if err := chromedp.Run(ctx, chromedp.Text("body", &resourceBody)); err != nil {
				return err
			}
		}

		// Try to determine MIME type from URL
		resourceMimeType = getMimeTypeFromURL(resourceURL)

		return nil
	}))

	if err != nil {
		log.Printf("Failed to fetch resource %s: %v", resourceURL, err)
		return "", getMimeTypeFromURL(resourceURL)
	}

	return resourceBody, resourceMimeType
}
