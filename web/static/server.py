#!/usr/bin/env python3
"""
Gthulhu Web UI Development Server with API Proxy
Serves static files and proxies API requests to avoid CORS issues.
"""

import http.server
import urllib.request
import urllib.error
import json
import os
import sys

# Configuration
STATIC_DIR = os.path.dirname(os.path.abspath(__file__))
API_BASE_URL = os.environ.get('API_BASE_URL', 'http://localhost:8080')
DECISION_MAKER_URL = os.environ.get('DECISION_MAKER_URL', 'http://localhost:8081')
PORT = int(os.environ.get('PORT', '3000'))

# Endpoints that should be routed to the Decision Maker instead of Manager
DECISION_MAKER_ENDPOINTS = [
    '/api/v1/pods/pids',
]

class ProxyHandler(http.server.SimpleHTTPRequestHandler):
    """HTTP handler that serves static files and proxies API requests."""
    
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=STATIC_DIR, **kwargs)
    
    def do_OPTIONS(self):
        """Handle CORS preflight requests."""
        self.send_response(204)
        self.send_cors_headers()
        self.end_headers()
    
    def do_GET(self):
        """Handle GET requests - proxy API or serve static files."""
        if self.path.startswith('/api/') or self.path.startswith('/health') or self.path.startswith('/version'):
            self.proxy_request('GET', self._get_backend_url())
        else:
            super().do_GET()
    
    def _get_backend_url(self):
        """Determine which backend to use based on the request path."""
        path_without_query = self.path.split('?')[0]
        for endpoint in DECISION_MAKER_ENDPOINTS:
            if path_without_query.startswith(endpoint):
                return DECISION_MAKER_URL
        return API_BASE_URL
    
    def do_POST(self):
        """Handle POST requests - proxy to API."""
        self.proxy_request('POST', self._get_backend_url())
    
    def do_PUT(self):
        """Handle PUT requests - proxy to API."""
        self.proxy_request('PUT', self._get_backend_url())
    
    def do_DELETE(self):
        """Handle DELETE requests - proxy to API."""
        self.proxy_request('DELETE', self._get_backend_url())
    
    def send_cors_headers(self):
        """Add CORS headers to response."""
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
        self.send_header('Access-Control-Max-Age', '86400')
    
    def proxy_request(self, method, backend_url=None):
        """Proxy request to the appropriate backend server."""
        if backend_url is None:
            backend_url = API_BASE_URL
        url = f"{backend_url}{self.path}"
        
        # Read request body for POST/PUT
        body = None
        if method in ('POST', 'PUT'):
            content_length = int(self.headers.get('Content-Length', 0))
            if content_length > 0:
                body = self.rfile.read(content_length)
        
        # Build request headers
        headers = {}
        if 'Content-Type' in self.headers:
            headers['Content-Type'] = self.headers['Content-Type']
        if 'Authorization' in self.headers:
            headers['Authorization'] = self.headers['Authorization']
        
        try:
            req = urllib.request.Request(url, data=body, headers=headers, method=method)
            with urllib.request.urlopen(req, timeout=30) as response:
                response_body = response.read()
                
                self.send_response(response.status)
                self.send_cors_headers()
                
                # Forward content-type
                content_type = response.headers.get('Content-Type', 'application/json')
                self.send_header('Content-Type', content_type)
                self.send_header('Content-Length', len(response_body))
                self.end_headers()
                
                self.wfile.write(response_body)
                
        except urllib.error.HTTPError as e:
            error_body = e.read()
            self.send_response(e.code)
            self.send_cors_headers()
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', len(error_body))
            self.end_headers()
            self.wfile.write(error_body)
            
        except urllib.error.URLError as e:
            error_msg = json.dumps({
                'success': False,
                'error': f'API server unreachable: {str(e.reason)}'
            }).encode()
            self.send_response(503)
            self.send_cors_headers()
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', len(error_msg))
            self.end_headers()
            self.wfile.write(error_msg)
            
        except Exception as e:
            error_msg = json.dumps({
                'success': False,
                'error': f'Proxy error: {str(e)}'
            }).encode()
            self.send_response(500)
            self.send_cors_headers()
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', len(error_msg))
            self.end_headers()
            self.wfile.write(error_msg)
    
    def log_message(self, format, *args):
        """Custom log format."""
        method = args[0].split()[0] if args else '?'
        path = args[0].split()[1] if args and len(args[0].split()) > 1 else '?'
        status = args[1] if len(args) > 1 else '?'
        
        # Color coding
        if str(status).startswith('2'):
            color = '\033[92m'  # Green
        elif str(status).startswith('3'):
            color = '\033[93m'  # Yellow
        elif str(status).startswith('4'):
            color = '\033[91m'  # Red
        elif str(status).startswith('5'):
            color = '\033[95m'  # Magenta
        else:
            color = '\033[0m'   # Default
        
        reset = '\033[0m'
        print(f"{color}[{method}]{reset} {path} → {color}{status}{reset}")


def main():
    print(f"""
\033[96m╔═══════════════════════════════════════════════════════════╗
║         Gthulhu Web UI Development Server                 ║
╚═══════════════════════════════════════════════════════════╝\033[0m

  \033[92m●\033[0m Static files:    {STATIC_DIR}
  \033[92m●\033[0m Manager API:     {API_BASE_URL}
  \033[92m●\033[0m DecisionMaker:   {DECISION_MAKER_URL}
  \033[92m●\033[0m Server:          http://localhost:{PORT}

  \033[93mPress Ctrl+C to stop\033[0m
""")
    
    # Use ThreadingHTTPServer to handle multiple concurrent requests
    class ThreadingHTTPServer(http.server.ThreadingHTTPServer):
        allow_reuse_address = True
        daemon_threads = True
    
    with ThreadingHTTPServer(('', PORT), ProxyHandler) as httpd:
        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\n\033[93mShutting down...\033[0m")
            sys.exit(0)


if __name__ == '__main__':
    main()
