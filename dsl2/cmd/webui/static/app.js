// 全局变量
let currentExecution = null;
let resultCounter = 0;

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    initializeApp();
});

function initializeApp() {
    // 绑定事件
    bindEvents();
    
    // 加载示例
    loadExamples();
    
    // 设置默认编辑器内容
    setDefaultWorkflow();
    
    // 刷新工作流列表
    refreshWorkflows();
}

function bindEvents() {
    // 按钮事件
    document.getElementById('executeBtn').addEventListener('click', executeWorkflow);
    document.getElementById('validateBtn').addEventListener('click', validateWorkflow);
    document.getElementById('clearResults').addEventListener('click', clearResults);
    document.getElementById('refreshWorkflows').addEventListener('click', refreshWorkflows);
    
    // 示例选择
    document.getElementById('exampleSelect').addEventListener('change', loadSelectedExample);
    
    // 编辑器变化
    document.getElementById('workflowEditor').addEventListener('input', onEditorChange);
}

function setDefaultWorkflow() {
    const defaultWorkflow = `version: "1.0"
taskQueue: "demo"
timeoutSec: 30
concurrency: 8
retry:
  maxAttempts: 3
  initialIntervalSec: 1
  maxIntervalSec: 10
  backoffCoefficient: 2.0

variables:
  x: 1
  y: 2

root:
  sequence:
    elements:
      - parallel:
          branches:
            - activity:
                name: "DoA"
                args: [{ ref: "x" }]
                result: "a"
            - activity:
                name: "DoB"
                args: [{ ref: "y" }]
                result: "b"
      - activity:
          name: "DoC"
          args: [{ ref: "a" }, { ref: "b" }]
          result: "c"`;
    
    document.getElementById('workflowEditor').value = defaultWorkflow;
    updateStatus('Ready to execute', 'info');
}

function loadExamples() {
    fetch('/api/examples')
        .then(response => response.json())
        .then(examples => {
            const select = document.getElementById('exampleSelect');
            select.innerHTML = '<option value="">Select Example...</option>';
            
            Object.keys(examples).forEach(name => {
                const option = document.createElement('option');
                option.value = name;
                option.textContent = name;
                select.appendChild(option);
            });
        })
        .catch(error => {
            console.error('Failed to load examples:', error);
        });
}

function loadSelectedExample() {
    const select = document.getElementById('exampleSelect');
    const selectedExample = select.value;
    
    if (!selectedExample) return;
    
    fetch('/api/examples')
        .then(response => response.json())
        .then(examples => {
            if (examples[selectedExample]) {
                document.getElementById('workflowEditor').value = examples[selectedExample];
                updateStatus(`Loaded example: ${selectedExample}`, 'info');
            }
        })
        .catch(error => {
            console.error('Failed to load example:', error);
            updateStatus('Failed to load example', 'error');
        });
}

function validateWorkflow() {
    const yaml = document.getElementById('workflowEditor').value;
    
    if (!yaml.trim()) {
        updateStatus('Please enter a workflow YAML', 'error');
        return;
    }
    
    updateStatus('Validating workflow...', 'info');
    
    // 简单的 YAML 语法检查
    try {
        // 基本的 YAML 结构检查
        if (!yaml.includes('version:') || !yaml.includes('taskQueue:') || !yaml.includes('root:')) {
            throw new Error('Missing required fields: version, taskQueue, or root');
        }
        
        updateStatus('Workflow YAML appears valid', 'success');
    } catch (error) {
        updateStatus(`Validation error: ${error.message}`, 'error');
    }
}

function executeWorkflow() {
    const yaml = document.getElementById('workflowEditor').value;
    
    if (!yaml.trim()) {
        updateStatus('Please enter a workflow YAML', 'error');
        return;
    }
    
    // 禁用执行按钮
    const executeBtn = document.getElementById('executeBtn');
    const originalText = executeBtn.textContent;
    executeBtn.disabled = true;
    executeBtn.innerHTML = '<span class="loading"></span> Executing...';
    
    updateStatus('Executing workflow...', 'info');
    
    fetch('/api/workflow/execute', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ yaml: yaml })
    })
    .then(response => response.json())
    .then(data => {
        addResult(data);
        
        if (data.success) {
            updateStatus(`Workflow executed successfully. ID: ${data.workflowId}`, 'success');
        } else {
            updateStatus(`Execution failed: ${data.error}`, 'error');
        }
    })
    .catch(error => {
        console.error('Execution error:', error);
        updateStatus(`Execution failed: ${error.message}`, 'error');
        addResult({
            success: false,
            error: error.message,
            workflowId: null
        });
    })
    .finally(() => {
        // 恢复执行按钮
        executeBtn.disabled = false;
        executeBtn.textContent = originalText;
        
        // 刷新工作流列表
        refreshWorkflows();
    });
}

function addResult(result) {
    const resultsContainer = document.getElementById('workflowResults');
    const resultItem = document.createElement('div');
    resultItem.className = `result-item ${result.success ? 'success' : 'error'}`;
    
    const timestamp = new Date().toLocaleString();
    resultCounter++;
    
    resultItem.innerHTML = `
        <div class="result-header">
            <strong>Execution #${resultCounter}</strong>
            <span class="result-time">${timestamp}</span>
        </div>
        <div class="result-content">
${result.success ? 
    `✅ Success
Workflow ID: ${result.workflowId}
Run ID: ${result.runId}

Result:
${formatJSON(result.result)}` :
    `❌ Failed
Error: ${result.error}`
}
        </div>
    `;
    
    resultsContainer.insertBefore(resultItem, resultsContainer.firstChild);
}

function formatJSON(obj) {
    if (!obj) return 'No result data';
    
    try {
        return JSON.stringify(obj, null, 2);
    } catch (error) {
        return String(obj);
    }
}

function clearResults() {
    document.getElementById('workflowResults').innerHTML = '';
    resultCounter = 0;
    updateStatus('Results cleared', 'info');
}

function refreshWorkflows() {
    const workflowsList = document.getElementById('workflowsList');
    
    // 显示加载状态
    workflowsList.innerHTML = '<p style="text-align: center; padding: 20px; color: #666;">Loading workflows...</p>';
    
    fetch('/api/workflow/list')
        .then(response => response.json())
        .then(workflows => {
            if (workflows.length === 0) {
                workflowsList.innerHTML = '<p style="text-align: center; padding: 20px; color: #666;">No recent workflows found</p>';
            } else {
                displayWorkflows(workflows);
            }
        })
        .catch(error => {
            console.error('Failed to load workflows:', error);
            workflowsList.innerHTML = '<p style="text-align: center; padding: 20px; color: #dc3545;">Failed to load workflows</p>';
        });
}

function displayWorkflows(workflows) {
    const workflowsList = document.getElementById('workflowsList');
    workflowsList.innerHTML = '';
    
    workflows.forEach(workflow => {
        const workflowItem = document.createElement('div');
        workflowItem.className = 'workflow-item';
        
        workflowItem.innerHTML = `
            <div class="workflow-info">
                <h4>${workflow.workflowId}</h4>
                <p>Status: ${workflow.status} | Started: ${new Date(workflow.startTime).toLocaleString()}</p>
            </div>
            <div class="workflow-actions">
                <button class="btn btn-secondary" onclick="viewWorkflow('${workflow.workflowId}')">
                    View
                </button>
                <button class="btn btn-primary" onclick="getWorkflowStatus('${workflow.workflowId}')">
                    Status
                </button>
            </div>
        `;
        
        workflowsList.appendChild(workflowItem);
    });
}

function viewWorkflow(workflowId) {
    // 这里可以实现查看工作流详情的功能
    alert(`View workflow: ${workflowId}\n\nThis feature will show detailed workflow information.`);
}

function getWorkflowStatus(workflowId) {
    updateStatus(`Querying status for ${workflowId}...`, 'info');
    
    fetch(`/api/workflow/status?id=${workflowId}`)
        .then(response => response.json())
        .then(status => {
            const result = {
                success: status.status !== 'Error',
                workflowId: status.workflowId,
                runId: status.runId,
                result: status.result,
                error: status.error
            };
            
            addResult(result);
            
            if (status.status === 'Error') {
                updateStatus(`Status query failed: ${status.error}`, 'error');
            } else {
                updateStatus(`Status retrieved for ${workflowId}`, 'success');
            }
        })
        .catch(error => {
            console.error('Status query error:', error);
            updateStatus(`Status query failed: ${error.message}`, 'error');
        });
}

function updateStatus(message, type) {
    const statusBar = document.getElementById('editorStatus');
    statusBar.textContent = message;
    statusBar.className = `status-bar status-${type}`;
    
    // 自动清除状态信息
    if (type !== 'error') {
        setTimeout(() => {
            if (statusBar.textContent === message) {
                statusBar.textContent = '';
                statusBar.className = 'status-bar';
            }
        }, 3000);
    }
}

function onEditorChange() {
    // 可以在这里添加实时验证或其他编辑器相关功能
    const yaml = document.getElementById('workflowEditor').value;
    if (yaml.trim()) {
        updateStatus('Modified', 'info');
    }
}

// 工具函数
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// 键盘快捷键
document.addEventListener('keydown', function(e) {
    if (e.ctrlKey || e.metaKey) {
        switch(e.key) {
            case 'Enter':
                e.preventDefault();
                executeWorkflow();
                break;
            case 's':
                e.preventDefault();
                validateWorkflow();
                break;
        }
    }
});

// 错误处理
window.addEventListener('error', function(e) {
    console.error('JavaScript error:', e.error);
    updateStatus('An unexpected error occurred', 'error');
});

// 网络状态监控
window.addEventListener('online', function() {
    updateStatus('Connection restored', 'success');
});

window.addEventListener('offline', function() {
    updateStatus('Connection lost - working offline', 'error');
});