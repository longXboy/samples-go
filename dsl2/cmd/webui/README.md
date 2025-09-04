# DSL Workflow Web UI

A beautiful and intuitive web interface for designing, executing, and monitoring your DSL workflows.

## Features

🎨 **Visual Editor**
- Syntax-highlighted YAML editor
- Real-time validation
- Built-in examples
- Responsive design

⚡ **Workflow Execution**
- One-click execution
- Real-time status updates
- Detailed results display
- Error handling and reporting

📊 **Monitoring**
- Recent workflows list
- Execution history
- Status querying
- Result visualization

🔧 **Developer-Friendly**
- RESTful API
- JSON responses
- Error details
- Keyboard shortcuts

## Quick Start

### 1. Prerequisites

- Go 1.21+
- Running Temporal server
- DSL worker running (see parent directory)

### 2. Start the Web Server

```bash
cd cmd/webui
go run main.go
```

### 3. Open in Browser

Navigate to: http://localhost:8080

## Usage Guide

### Creating Workflows

1. **Use Examples**: Select from predefined examples in the dropdown
2. **Write YAML**: Create your workflow using the DSL syntax
3. **Validate**: Click "Validate" to check syntax
4. **Execute**: Click "Execute" to run the workflow

### Example Workflows

The UI includes several built-in examples:
- **Basic Parallel**: Parallel execution and result merging
- **Map with Collection**: Concurrent processing with result collection
- **Conditional Branch**: If-else logic
- **While Loop**: Conditional loops

### Keyboard Shortcuts

- `Ctrl/Cmd + Enter`: Execute workflow
- `Ctrl/Cmd + S`: Validate workflow

## API Endpoints

### Execute Workflow
```
POST /api/workflow/execute
Body: {"yaml": "workflow yaml content"}
Response: {"success": true, "workflowId": "...", "result": {...}}
```

### Get Workflow Status
```
GET /api/workflow/status?id=workflow-id
Response: {"workflowId": "...", "status": "...", "result": {...}}
```

### List Workflows
```
GET /api/workflow/list
Response: [{"workflowId": "...", "status": "...", "startTime": "..."}]
```

### Get Examples
```
GET /api/examples
Response: {"Example Name": "yaml content", ...}
```

## Architecture

```
┌─────────────────┐    HTTP     ┌─────────────────┐    Temporal    ┌─────────────────┐
│   Web Browser   │ ◄──────────► │   Web Server    │ ◄──────────────► │ Temporal Server │
│                 │              │                 │                │                 │
│ - HTML/CSS/JS   │              │ - Go HTTP API   │                │ - Workflows     │
│ - YAML Editor   │              │ - YAML Parser   │                │ - Activities    │
│ - Result View   │              │ - DSL Executor  │                │ - Task Queue    │
└─────────────────┘              └─────────────────┘                └─────────────────┘
```

## File Structure

```
webui/
├── main.go              # Web server with API endpoints
├── static/
│   ├── style.css        # Modern, responsive styling
│   └── app.js           # Frontend JavaScript logic
├── go.mod               # Go module dependencies
└── README.md           # This file
```

## Customization

### Adding New Examples

Edit the `handleExamples` function in `main.go` to add new workflow examples.

### Styling

Modify `static/style.css` to customize the appearance. The UI uses:
- CSS Grid for layout
- Flexbox for components
- CSS gradients for visual appeal
- Responsive design patterns

### API Extensions

Add new endpoints in `main.go`:
1. Define handler function
2. Register route with `http.HandleFunc`
3. Update frontend JavaScript as needed

## Troubleshooting

### Connection Issues
- Ensure Temporal server is running on localhost:7233
- Check that DSL worker is registered and running
- Verify the taskQueue matches between UI and worker

### Execution Failures
- Check workflow YAML syntax
- Verify all referenced activities are registered
- Check worker logs for detailed error messages

### UI Issues
- Clear browser cache
- Check browser console for JavaScript errors
- Ensure all static files are served correctly

## Development

### Local Development
```bash
# Watch for changes (install air: go install github.com/cosmtrek/air@latest)
air

# Or run directly
go run main.go
```

### Building for Production
```bash
go build -o webui main.go
./webui
```

## Security Notes

This is a development/demo interface. For production use, consider:
- Adding authentication
- Input validation and sanitization
- Rate limiting
- HTTPS/TLS
- CORS configuration
- Request logging