// 全局变量
let workflowData = {
    nodes: new Map(),
    connections: [],
    nextNodeId: 1
};

let selectedNode = null;
let draggedNode = null;
let canvas = null;
let contextMenu = null;

// 节点类型定义
const NODE_TYPES = {
    start: {
        title: 'Start',
        icon: 'fas fa-play-circle',
        color: '#4CAF50',
        inputs: 0,
        outputs: 1,
        properties: {}
    },
    end: {
        title: 'End', 
        icon: 'fas fa-stop-circle',
        color: '#f44336',
        inputs: 1,
        outputs: 0,
        properties: {}
    },
    activity: {
        title: 'Activity',
        icon: 'fas fa-cog',
        color: '#667eea',
        inputs: 1,
        outputs: 1,
        properties: {
            name: { type: 'text', label: 'Activity Name', required: true },
            args: { type: 'textarea', label: 'Arguments (JSON)', placeholder: '[]' },
            result: { type: 'text', label: 'Result Variable' },
            timeout: { type: 'number', label: 'Timeout (seconds)' }
        }
    },
    parallel: {
        title: 'Parallel',
        icon: 'fas fa-code-branch',
        color: '#2196f3',
        inputs: 1,
        outputs: 1, // parallel执行完毕后有一个输出，用于连接后续节点
        properties: {
            // 不再需要branches属性，因为并行分支由连接决定
        }
    },
    if: {
        title: 'If/Else',
        icon: 'fas fa-question',
        color: '#ff9800',
        inputs: 1,
        outputs: 2, // then, else
        properties: {
            condition: { type: 'textarea', label: 'Condition (YAML)', required: true, 
                        placeholder: 'eq:\n  left: { ref: "variable" }\n  right: { str: "value" }' },
            description: { type: 'text', label: 'Description' }
        }
    },
    while: {
        title: 'While Loop',
        icon: 'fas fa-sync',
        color: '#9c27b0',
        inputs: 1,
        outputs: 2, // continue, exit
        properties: {
            condition: { type: 'textarea', label: 'Loop Condition (YAML)', required: true },
            maxIters: { type: 'number', label: 'Max Iterations', default: 10 },
            sleepSeconds: { type: 'number', label: 'Sleep Between Iterations (sec)', default: 0 }
        }
    },
    map: {
        title: 'Map',
        icon: 'fas fa-list',
        color: '#607d8b',
        inputs: 1,
        outputs: 1,
        properties: {
            itemsRef: { type: 'text', label: 'Items Variable', required: true },
            itemVar: { type: 'text', label: 'Item Variable Name', default: '_item' },
            concurrency: { type: 'number', label: 'Concurrency', default: 1 },
            collectVar: { type: 'text', label: 'Collect Variable' },
            failFast: { type: 'checkbox', label: 'Fail Fast', default: true }
        }
    }
};

// DOM 初始化
document.addEventListener('DOMContentLoaded', function() {
    initializeApp();
});

function initializeApp() {
    canvas = document.getElementById('canvasContent');
    contextMenu = document.getElementById('contextMenu');
    
    setupEventListeners();
    loadExamples();
    
    // 创建默认的开始节点
    createNode('start', { x: 100, y: 200 });
    
    updateStatus('Ready - Drag nodes from the palette to build your workflow');
}

function setupEventListeners() {
    // 工具栏按钮
    document.getElementById('validateBtn').addEventListener('click', validateWorkflow);
    document.getElementById('executeBtn').addEventListener('click', executeWorkflow);
    document.getElementById('saveBtn').addEventListener('click', saveWorkflow);
    document.getElementById('exampleSelect').addEventListener('change', loadSelectedExample);
    
    // 底部面板控制
    document.getElementById('toggleResults').addEventListener('click', toggleResultsPanel);
    document.getElementById('toggleYaml').addEventListener('click', toggleYamlPanel);
    
    // 标签页切换
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            switchTab(e.target.dataset.tab);
        });
    });
    
    // 画布事件
    canvas.addEventListener('dragover', handleCanvasDragOver);
    canvas.addEventListener('drop', handleCanvasDrop);
    canvas.addEventListener('click', handleCanvasClick);
    canvas.addEventListener('contextmenu', handleCanvasRightClick);
    
    // 节点面板拖拽
    document.querySelectorAll('.palette-node').forEach(node => {
        node.addEventListener('dragstart', handlePaletteDragStart);
    });
    
    // 右键菜单
    document.addEventListener('click', hideContextMenu);
    document.querySelectorAll('.menu-item').forEach(item => {
        item.addEventListener('click', handleContextMenuAction);
    });
    
    // 模态框
    const modal = document.getElementById('nodeEditModal');
    document.getElementById('modalClose').addEventListener('click', () => closeModal());
    document.getElementById('modalCancel').addEventListener('click', () => closeModal());
    document.getElementById('modalSave').addEventListener('click', saveNodeProperties);
    
    // 点击模态框背景关闭
    modal.addEventListener('click', (e) => {
        if (e.target === modal) closeModal();
    });
    
    // 键盘快捷键
    document.addEventListener('keydown', handleKeyboard);
}

// 拖拽处理
function handlePaletteDragStart(e) {
    const nodeType = e.target.dataset.type;
    e.dataTransfer.setData('text/plain', nodeType);
    e.dataTransfer.effectAllowed = 'copy';
    
    // 创建拖拽预览
    const preview = e.target.cloneNode(true);
    preview.classList.add('drag-preview');
    document.body.appendChild(preview);
    
    setTimeout(() => {
        if (document.body.contains(preview)) {
            document.body.removeChild(preview);
        }
    }, 0);
}

function handleCanvasDragOver(e) {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'copy';
}

function handleCanvasDrop(e) {
    e.preventDefault();
    const nodeType = e.dataTransfer.getData('text/plain');
    
    if (nodeType && NODE_TYPES[nodeType]) {
        const rect = canvas.getBoundingClientRect();
        const x = e.clientX - rect.left - canvas.scrollLeft;
        const y = e.clientY - rect.top - canvas.scrollTop;
        
        createNode(nodeType, { x, y });
    }
}

// 节点创建和管理
function createNode(type, position, properties = {}) {
    const nodeId = `node_${workflowData.nextNodeId++}`;
    const nodeType = NODE_TYPES[type];
    
    // 合并默认属性
    const nodeProperties = {};
    if (nodeType.properties) {
        Object.keys(nodeType.properties).forEach(key => {
            const prop = nodeType.properties[key];
            nodeProperties[key] = properties[key] || prop.default || '';
        });
    }
    
    const nodeData = {
        id: nodeId,
        type: type,
        title: nodeType.title,
        position: position,
        properties: nodeProperties
    };
    
    workflowData.nodes.set(nodeId, nodeData);
    
    // 创建DOM元素
    const nodeElement = createNodeElement(nodeData);
    canvas.appendChild(nodeElement);
    
    updateStatus(`Added ${nodeType.title} node`);
    return nodeData;
}

function createNodeElement(nodeData) {
    const nodeType = NODE_TYPES[nodeData.type];
    const element = document.createElement('div');
    
    element.className = 'workflow-node';
    element.dataset.nodeId = nodeData.id;
    element.dataset.type = nodeData.type;
    element.style.left = nodeData.position.x + 'px';
    element.style.top = nodeData.position.y + 'px';
    
    // 节点内容
    let configText = '';
    if (nodeData.properties && Object.keys(nodeData.properties).length > 0) {
        const configs = Object.entries(nodeData.properties)
            .filter(([key, value]) => value && value.toString().trim())
            .map(([key, value]) => `${key}: ${value}`);
        if (configs.length > 0) {
            configText = `<div class="node-config">${configs.join(', ')}</div>`;
        }
    }
    
    element.innerHTML = `
        <div class="node-header">
            <div class="node-icon">
                <i class="${nodeType.icon}"></i>
            </div>
            <div class="node-title">${nodeData.title}</div>
        </div>
        <div class="node-body">
            ${getNodeDescription(nodeData)}
            ${configText}
        </div>
    `;
    
    // 添加连接点
    if (nodeType.inputs > 0) {
        const inputPoint = document.createElement('div');
        inputPoint.className = 'connection-point input';
        inputPoint.dataset.nodeId = nodeData.id;
        inputPoint.dataset.type = 'input';
        element.appendChild(inputPoint);
    }
    
    if (nodeType.outputs > 0) {
        const outputPoint = document.createElement('div');
        outputPoint.className = 'connection-point output';
        outputPoint.dataset.nodeId = nodeData.id;
        outputPoint.dataset.type = 'output';
        element.appendChild(outputPoint);
    }
    
    // 事件监听器
    element.addEventListener('click', (e) => {
        e.stopPropagation();
        selectNode(nodeData.id);
    });
    
    element.addEventListener('dblclick', (e) => {
        e.stopPropagation();
        editNode(nodeData.id);
    });
    
    // 拖拽移动和连接处理
    let isDragging = false;
    let dragStart = { x: 0, y: 0 };
    let isConnectionDrag = false;
    
    element.addEventListener('mousedown', (e) => {
        // 检查是否点击了连接点
        if (e.target.classList.contains('connection-point')) {
            const pointType = e.target.dataset.type;
            const nodeId = e.target.dataset.nodeId;
            
            if (pointType === 'output') {
                // 开始连接
                startConnection(e.target, e);
                isConnectionDrag = true;
                e.stopPropagation();
                e.preventDefault();
                return;
            }
            return;
        }
        
        // 普通节点拖拽
        isDragging = true;
        dragStart.x = e.clientX - nodeData.position.x;
        dragStart.y = e.clientY - nodeData.position.y;
        
        element.classList.add('dragging');
        e.preventDefault();
    });
    
    document.addEventListener('mousemove', (e) => {
        if (isConnectionDrag) {
            updateTempConnection(e);
            return;
        }
        
        if (!isDragging || element.dataset.nodeId !== nodeData.id) return;
        
        const newX = e.clientX - dragStart.x;
        const newY = e.clientY - dragStart.y;
        
        nodeData.position.x = Math.max(0, newX);
        nodeData.position.y = Math.max(0, newY);
        
        element.style.left = nodeData.position.x + 'px';
        element.style.top = nodeData.position.y + 'px';
        
        updateConnections();
    });
    
    document.addEventListener('mouseup', (e) => {
        if (isConnectionDrag) {
            finishConnection(e);
            isConnectionDrag = false;
            return;
        }
        
        if (isDragging && element.dataset.nodeId === nodeData.id) {
            isDragging = false;
            element.classList.remove('dragging');
        }
    });
    
    return element;
}

function getNodeDescription(nodeData) {
    const type = nodeData.type;
    const props = nodeData.properties;
    
    switch(type) {
        case 'activity':
            return props.name || 'Click to configure activity';
        case 'if':
            return props.description || 'Conditional branch';
        case 'while':
            return `Loop with max ${props.maxIters || 10} iterations`;
        case 'map':
            return `Process ${props.itemsRef || 'items'} with concurrency ${props.concurrency || 1}`;
        case 'parallel':
            return `Execute multiple branches in parallel`;
        default:
            return NODE_TYPES[type]?.title || type;
    }
}

// 节点选择
function selectNode(nodeId) {
    // 清除之前的选择
    document.querySelectorAll('.workflow-node.selected').forEach(node => {
        node.classList.remove('selected');
    });
    
    const nodeElement = document.querySelector(`[data-node-id="${nodeId}"]`);
    if (nodeElement) {
        nodeElement.classList.add('selected');
        selectedNode = nodeId;
        showNodeProperties(nodeId);
    }
}

function showNodeProperties(nodeId) {
    const nodeData = workflowData.nodes.get(nodeId);
    if (!nodeData) return;
    
    const propertiesContent = document.getElementById('propertiesContent');
    const nodeType = NODE_TYPES[nodeData.type];
    
    let html = `
        <div class="property-group">
            <h4>Node Information</h4>
            <div class="form-field">
                <label class="form-label">Type</label>
                <input type="text" class="form-input" value="${nodeType.title}" readonly>
            </div>
            <div class="form-field">
                <label class="form-label">ID</label>
                <input type="text" class="form-input" value="${nodeData.id}" readonly>
            </div>
        </div>
    `;
    
    if (nodeType.properties && Object.keys(nodeType.properties).length > 0) {
        html += `
            <div class="property-group">
                <h4>Properties</h4>
        `;
        
        Object.entries(nodeType.properties).forEach(([key, prop]) => {
            const value = nodeData.properties[key] || '';
            html += createPropertyField(key, prop, value);
        });
        
        html += `</div>`;
        
        html += `
            <div class="property-group">
                <button class="btn btn-primary" onclick="editNode('${nodeId}')">
                    <i class="fas fa-edit"></i> Edit Properties
                </button>
            </div>
        `;
    }
    
    propertiesContent.innerHTML = html;
}

function createPropertyField(key, prop, value) {
    let input = '';
    
    switch(prop.type) {
        case 'text':
            input = `<input type="text" class="form-input" value="${value}" readonly>`;
            break;
        case 'number':
            input = `<input type="number" class="form-input" value="${value}" readonly>`;
            break;
        case 'textarea':
            input = `<textarea class="form-textarea" readonly>${value}</textarea>`;
            break;
        case 'checkbox':
            input = `<input type="checkbox" ${value ? 'checked' : ''} disabled>`;
            break;
        default:
            input = `<input type="text" class="form-input" value="${value}" readonly>`;
    }
    
    return `
        <div class="form-field">
            <label class="form-label">${prop.label}</label>
            ${input}
        </div>
    `;
}

// 节点编辑
function editNode(nodeId) {
    const nodeData = workflowData.nodes.get(nodeId);
    if (!nodeData) return;
    
    const nodeType = NODE_TYPES[nodeData.type];
    const modal = document.getElementById('nodeEditModal');
    const modalTitle = document.getElementById('modalTitle');
    const modalBody = document.getElementById('modalBody');
    
    modalTitle.textContent = `Edit ${nodeType.title}`;
    
    let html = '';
    
    if (nodeType.properties && Object.keys(nodeType.properties).length > 0) {
        Object.entries(nodeType.properties).forEach(([key, prop]) => {
            const value = nodeData.properties[key] || prop.default || '';
            html += createEditablePropertyField(key, prop, value);
        });
    } else {
        html = '<p>This node type has no configurable properties.</p>';
    }
    
    modalBody.innerHTML = html;
    modal.classList.add('show');
    
    // 存储当前编辑的节点ID
    modal.dataset.editingNode = nodeId;
}

function createEditablePropertyField(key, prop, value) {
    let input = '';
    const required = prop.required ? 'required' : '';
    const placeholder = prop.placeholder ? `placeholder="${prop.placeholder}"` : '';
    
    switch(prop.type) {
        case 'text':
            input = `<input type="text" class="form-input" name="${key}" value="${value}" ${placeholder} ${required}>`;
            break;
        case 'number':
            const min = prop.min !== undefined ? `min="${prop.min}"` : '';
            const max = prop.max !== undefined ? `max="${prop.max}"` : '';
            input = `<input type="number" class="form-input" name="${key}" value="${value}" ${min} ${max} ${required}>`;
            break;
        case 'textarea':
            input = `<textarea class="form-textarea" name="${key}" ${placeholder} ${required}>${value}</textarea>`;
            break;
        case 'checkbox':
            input = `<input type="checkbox" name="${key}" ${value ? 'checked' : ''}>`;
            break;
        default:
            input = `<input type="text" class="form-input" name="${key}" value="${value}" ${placeholder} ${required}>`;
    }
    
    return `
        <div class="form-field">
            <label class="form-label">${prop.label}</label>
            ${input}
        </div>
    `;
}

function saveNodeProperties() {
    const modal = document.getElementById('nodeEditModal');
    const nodeId = modal.dataset.editingNode;
    const nodeData = workflowData.nodes.get(nodeId);
    
    if (!nodeData) return;
    
    // 获取表单数据
    const modalBody = modal.querySelector('.modal-body');
    const inputs = modalBody.querySelectorAll('input, textarea, select');
    
    // 更新节点属性
    Object.keys(NODE_TYPES[nodeData.type].properties || {}).forEach(key => {
        const prop = NODE_TYPES[nodeData.type].properties[key];
        const input = modalBody.querySelector(`[name="${key}"]`);
        
        if (input) {
            if (prop.type === 'checkbox') {
                nodeData.properties[key] = input.checked;
            } else {
                nodeData.properties[key] = input.value || '';
            }
        }
    });
    
    // 更新DOM
    const nodeElement = document.querySelector(`[data-node-id="${nodeId}"]`);
    if (nodeElement) {
        // 重新创建节点内容
        const newElement = createNodeElement(nodeData);
        nodeElement.parentNode.replaceChild(newElement, nodeElement);
        
        // 重新选择节点
        setTimeout(() => selectNode(nodeId), 10);
    }
    
    closeModal();
    updateStatus('Node properties updated');
    generateYAML();
}

function closeModal() {
    const modal = document.getElementById('nodeEditModal');
    modal.classList.remove('show');
    delete modal.dataset.editingNode;
}

// 画布交互
function handleCanvasClick(e) {
    if (e.target === canvas || e.target.classList.contains('canvas-content')) {
        // 点击空白区域，取消选择
        document.querySelectorAll('.workflow-node.selected').forEach(node => {
            node.classList.remove('selected');
        });
        selectedNode = null;
        
        const propertiesContent = document.getElementById('propertiesContent');
        propertiesContent.innerHTML = `
            <div class="no-selection">
                <i class="fas fa-mouse-pointer"></i>
                <p>Select a node to edit its properties</p>
            </div>
        `;
    }
}

function handleCanvasRightClick(e) {
    e.preventDefault();
    
    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    
    contextMenu.style.left = e.clientX + 'px';
    contextMenu.style.top = e.clientY + 'px';
    contextMenu.style.display = 'block';
    
    // 存储右键位置
    contextMenu.dataset.x = x;
    contextMenu.dataset.y = y;
}

function hideContextMenu() {
    contextMenu.style.display = 'none';
}

function handleContextMenuAction(e) {
    const action = e.target.dataset.action || e.target.parentElement.dataset.action;
    
    switch(action) {
        case 'delete':
            if (selectedNode) {
                deleteNode(selectedNode);
            }
            break;
        case 'copy':
            if (selectedNode) {
                copyNode(selectedNode);
            }
            break;
        case 'edit':
            if (selectedNode) {
                editNode(selectedNode);
            }
            break;
        case 'disconnect':
            if (selectedNode) {
                disconnectNode(selectedNode);
            }
            break;
    }
    
    hideContextMenu();
}

function deleteNode(nodeId) {
    const nodeData = workflowData.nodes.get(nodeId);
    if (!nodeData) return;
    
    // 删除相关连接
    workflowData.connections = workflowData.connections.filter(conn => 
        conn.from !== nodeId && conn.to !== nodeId
    );
    
    // 删除节点数据
    workflowData.nodes.delete(nodeId);
    
    // 删除DOM元素
    const nodeElement = document.querySelector(`[data-node-id="${nodeId}"]`);
    if (nodeElement) {
        nodeElement.remove();
    }
    
    // 清除选择
    if (selectedNode === nodeId) {
        selectedNode = null;
        const propertiesContent = document.getElementById('propertiesContent');
        propertiesContent.innerHTML = `
            <div class="no-selection">
                <i class="fas fa-mouse-pointer"></i>
                <p>Select a node to edit its properties</p>
            </div>
        `;
    }
    
    updateStatus('Node deleted');
    updateConnections();
    generateYAML();
}

function disconnectNode(nodeId) {
    const nodeData = workflowData.nodes.get(nodeId);
    if (!nodeData) return;
    
    // 删除与此节点相关的所有连接
    const connectionsToRemove = workflowData.connections.filter(conn => 
        conn.from === nodeId || conn.to === nodeId
    );
    
    workflowData.connections = workflowData.connections.filter(conn => 
        conn.from !== nodeId && conn.to !== nodeId
    );
    
    updateStatus(`Disconnected ${connectionsToRemove.length} connections from node`);
    updateConnections();
    generateYAML();
}

// 连接处理
let isConnecting = false;
let connectionStart = null;
let tempConnection = null;

function startConnection(outputPoint, e) {
    isConnecting = true;
    connectionStart = {
        nodeId: outputPoint.dataset.nodeId,
        element: outputPoint
    };
    
    outputPoint.classList.add('connecting');
    canvas.style.cursor = 'crosshair';
    
    // 创建临时连接线
    createTempConnection(e);
    updateStatus('Drag to an input point to create connection');
}

function finishConnection(e) {
    if (!isConnecting) return;
    
    // 检查鼠标下的元素
    const elementUnder = document.elementFromPoint(e.clientX, e.clientY);
    
    if (elementUnder && elementUnder.classList.contains('connection-point') && 
        elementUnder.dataset.type === 'input') {
        
        const targetNodeId = elementUnder.dataset.nodeId;
        
        // 不能连接到同一个节点
        if (targetNodeId === connectionStart.nodeId) {
            updateStatus('Cannot connect node to itself');
            cancelConnection();
            return;
        }
        
        // 检查是否已经存在连接到目标节点
        const existingConnection = workflowData.connections.find(conn => 
            conn.to === targetNodeId
        );
        
        if (existingConnection) {
            updateStatus('Target node already has a connection');
            cancelConnection();
            return;
        }
        
        // 创建连接
        const connection = {
            from: connectionStart.nodeId,
            to: targetNodeId,
            id: `conn_${Date.now()}`
        };
        
        workflowData.connections.push(connection);
        updateStatus(`Connected ${connectionStart.nodeId} to ${targetNodeId}`);
        finishConnectionSuccess();
    } else {
        cancelConnection();
    }
}

function createTempConnection(e) {
    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left + canvas.scrollLeft;
    const y = e.clientY - rect.top + canvas.scrollTop;
    
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.classList.add('temp-connection');
    svg.style.position = 'absolute';
    svg.style.top = '0';
    svg.style.left = '0';
    svg.style.width = '100%';
    svg.style.height = '100%';
    svg.style.pointerEvents = 'none';
    svg.style.zIndex = '999';
    
    const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    path.setAttribute('stroke', '#ff9800');
    path.setAttribute('stroke-width', '3');
    path.setAttribute('fill', 'none');
    path.setAttribute('stroke-dasharray', '8,4');
    
    const startRect = connectionStart.element.getBoundingClientRect();
    const startX = startRect.right - rect.left + canvas.scrollLeft - 6;
    const startY = startRect.top + startRect.height / 2 - rect.top + canvas.scrollTop;
    
    path.setAttribute('d', `M ${startX} ${startY} L ${x} ${y}`);
    
    svg.appendChild(path);
    canvas.appendChild(svg);
    
    tempConnection = {
        svg: svg,
        path: path,
        startX: startX,
        startY: startY
    };
}

function updateTempConnection(e) {
    if (!tempConnection) return;
    
    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left + canvas.scrollLeft;
    const y = e.clientY - rect.top + canvas.scrollTop;
    
    const controlOffset = Math.abs(x - tempConnection.startX) * 0.3;
    const pathData = `M ${tempConnection.startX} ${tempConnection.startY} C ${tempConnection.startX + controlOffset} ${tempConnection.startY}, ${x - controlOffset} ${y}, ${x} ${y}`;
    tempConnection.path.setAttribute('d', pathData);
    
    // 高亮潜在的目标连接点
    const elementUnder = document.elementFromPoint(e.clientX, e.clientY);
    document.querySelectorAll('.connection-point.highlight').forEach(el => {
        el.classList.remove('highlight');
    });
    
    if (elementUnder && elementUnder.classList.contains('connection-point') && 
        elementUnder.dataset.type === 'input' &&
        elementUnder.dataset.nodeId !== connectionStart.nodeId) {
        elementUnder.classList.add('highlight');
    }
}

function cancelConnection() {
    cleanupConnection();
    updateStatus('Connection cancelled');
}

function finishConnectionSuccess() {
    cleanupConnection();
    updateConnections();
    generateYAML();
}

function cleanupConnection() {
    isConnecting = false;
    connectionStart = null;
    canvas.style.cursor = 'default';
    
    // 清除连接状态
    document.querySelectorAll('.connection-point.connecting').forEach(point => {
        point.classList.remove('connecting');
    });
    
    document.querySelectorAll('.connection-point.highlight').forEach(point => {
        point.classList.remove('highlight');
    });
    
    // 移除临时连接线
    if (tempConnection) {
        tempConnection.svg.remove();
        tempConnection = null;
    }
}

function updateConnections() {
    // 清除现有连接线
    document.querySelectorAll('.connection-line').forEach(line => line.remove());
    
    // 重新绘制所有连接
    workflowData.connections.forEach(connection => {
        drawConnection(connection);
    });
}

function drawConnection(connection) {
    const fromElement = document.querySelector(`[data-node-id="${connection.from}"]`);
    const toElement = document.querySelector(`[data-node-id="${connection.to}"]`);
    
    if (!fromElement || !toElement) return;
    
    const fromRect = fromElement.getBoundingClientRect();
    const toRect = toElement.getBoundingClientRect();
    const canvasRect = canvas.getBoundingClientRect();
    
    const x1 = fromRect.right - canvasRect.left + canvas.scrollLeft - 6;
    const y1 = fromRect.top + fromRect.height / 2 - canvasRect.top + canvas.scrollTop;
    const x2 = toRect.left - canvasRect.left + canvas.scrollLeft + 6;
    const y2 = toRect.top + toRect.height / 2 - canvasRect.top + canvas.scrollTop;
    
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.classList.add('connection-line');
    svg.style.position = 'absolute';
    svg.style.top = '0';
    svg.style.left = '0';
    svg.style.width = '100%';
    svg.style.height = '100%';
    svg.style.pointerEvents = 'none';
    svg.style.zIndex = '1';
    
    const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    path.classList.add('connection-path');
    
    // 创建贝塞尔曲线路径
    const controlOffset = Math.abs(x2 - x1) * 0.5;
    const pathData = `M ${x1} ${y1} C ${x1 + controlOffset} ${y1}, ${x2 - controlOffset} ${y2}, ${x2} ${y2}`;
    path.setAttribute('d', pathData);
    
    svg.appendChild(path);
    canvas.appendChild(svg);
}

// 工作流生成和验证
function generateYAML() {
    // 简化版本的YAML生成
    const workflow = {
        version: "1.0",
        taskQueue: "demo",
        timeoutSec: 30,
        variables: {},
        root: [] // 直接是数组，不再是sequence包装
    };
    
    console.log("Starting YAML generation, nodes:", workflowData.nodes.size, "connections:", workflowData.connections.length);
    
    // 如果没有节点，生成一个默认的示例
    if (workflowData.nodes.size === 0) {
        console.log("No nodes found, generating default example");
        workflow.root = [
            {
                activity: {
                    name: "DoA",
                    args: [{ int: 42 }],
                    result: "result"
                }
            }
        ];
    } else {
        // 寻找开始节点并按连接顺序构建root数组
        let startNode = null;
        for (let [id, node] of workflowData.nodes) {
            if (node.type === 'start') {
                startNode = node;
                break;
            }
        }
        
        console.log("Start node found:", startNode ? startNode.id : "none");
        
        if (startNode) {
            // 从开始节点开始，构建连接的Statement序列
            workflow.root = buildStatementSequence(startNode);
        } else {
            // 如果没有start节点，但有其他节点，生成所有非start/end节点
            console.log("No start node, generating from all activity nodes");
            for (let [id, node] of workflowData.nodes) {
                if (node.type !== 'start' && node.type !== 'end') {
                    const statement = buildNodeStructure(node);
                    if (statement) {
                        workflow.root.push(statement);
                    }
                }
            }
        }
    }
    
    console.log("Generated root with", workflow.root.length, "statements");
    
    // 生成简单的YAML字符串
    const yamlString = generateSimpleYAML(workflow);
    document.getElementById('yamlEditor').value = yamlString;
    
    return workflow;
}

function generateSimpleYAML(obj) {
    let yaml = `version: "${obj.version}"
taskQueue: "${obj.taskQueue}"
timeoutSec: ${obj.timeoutSec}
variables: {}
root:`;

    if (obj.root && obj.root.length > 0) {
        // 为每个语句生成YAML
        for (let statement of obj.root) {
            yaml += '\n  - ';
            if (statement.activity) {
                yaml += `activity:
      name: "${statement.activity.name || 'UnnamedActivity'}"
      args: ${JSON.stringify(statement.activity.args || [])}
      result: "${statement.activity.result || 'result'}"`;
            } else if (statement.parallel) {
                yaml += `parallel: ${JSON.stringify(statement.parallel)}`;
            } else {
                yaml += `activity:
      name: "Placeholder"
      result: "result"`;
            }
        }
    } else {
        // 生成默认示例
        yaml += `
  - activity:
      name: "DoA"
      args: [{ int: 42 }]
      result: "result"`;
    }
    
    return yaml;
}

function buildStatementSequence(startNode) {
    console.log("Building statement sequence from start node:", startNode.id);
    
    // 构建拓扑排序来确保正确的执行顺序
    function topologicalSort() {
        const graph = new Map();
        const inDegree = new Map();
        const allNodes = new Set();
        
        // 初始化图结构
        for (let [id, node] of workflowData.nodes) {
            if (node.type !== 'start' && node.type !== 'end') {
                graph.set(id, []);
                inDegree.set(id, 0);
                allNodes.add(id);
            }
        }
        
        // 构建连接关系
        for (let conn of workflowData.connections) {
            const fromNode = workflowData.nodes.get(conn.from);
            const toNode = workflowData.nodes.get(conn.to);
            
            if (fromNode && toNode && 
                fromNode.type !== 'end' && toNode.type !== 'end' &&
                toNode.type !== 'start') {
                
                // 确保from节点在图中（除了start节点）
                if (fromNode.type !== 'start') {
                    if (!graph.has(conn.from)) {
                        graph.set(conn.from, []);
                        inDegree.set(conn.from, 0);
                        allNodes.add(conn.from);
                    }
                    // 添加边
                    if (!graph.get(conn.from).includes(conn.to)) {
                        graph.get(conn.from).push(conn.to);
                        inDegree.set(conn.to, inDegree.get(conn.to) + 1);
                    }
                } else {
                    // start节点的连接不影响图结构，只设置目标节点入度为0
                    // 这部分逻辑在后面的start连接处理中完成
                }
            }
        }
        
        // 找出所有从start节点直接连接的节点作为入口
        const startConnections = workflowData.connections.filter(conn => conn.from === startNode.id);
        for (let conn of startConnections) {
            const toNode = workflowData.nodes.get(conn.to);
            if (toNode && toNode.type !== 'end') {
                inDegree.set(conn.to, 0); // 从start连接的节点入度设为0
            }
        }
        
        console.log("Graph structure:", Array.from(graph.entries()));
        console.log("In-degrees:", Array.from(inDegree.entries()));
        
        // Kahn算法进行拓扑排序
        const queue = [];
        const result = [];
        
        // 找到所有入度为0的节点
        for (let [nodeId, degree] of inDegree) {
            if (degree === 0) {
                queue.push(nodeId);
            }
        }
        
        console.log("Initial queue (zero in-degree nodes):", queue);
        
        while (queue.length > 0) {
            const current = queue.shift();
            const node = workflowData.nodes.get(current);
            
            if (node) {
                result.push(node);
                console.log(`Added node to result: ${current} (${node.type})`);
                
                // 减少邻接节点的入度
                const neighbors = graph.get(current) || [];
                for (let neighbor of neighbors) {
                    inDegree.set(neighbor, inDegree.get(neighbor) - 1);
                    if (inDegree.get(neighbor) === 0) {
                        queue.push(neighbor);
                        console.log(`Added to queue: ${neighbor}`);
                    }
                }
            }
        }
        
        console.log(`Topological sort result: ${result.length} nodes`);
        return result;
    }
    
    // 获取排序后的节点
    const sortedNodes = topologicalSort();
    const statements = [];
    
    // 按拓扑顺序构建语句
    for (let node of sortedNodes) {
        const statement = buildNodeStructure(node);
        if (statement) {
            statements.push(statement);
            console.log(`Added statement for node: ${node.id} (${node.type})`);
        }
    }
    
    console.log(`Built ${statements.length} statements total`);
    return statements;
}

function buildNodeStructure(node) {
    // 简化的节点结构生成
    switch(node.type) {
        case 'activity':
            return {
                activity: {
                    name: node.properties.name || 'UnnamedActivity',
                    args: parseJSONSafely(node.properties.args) || [],
                    result: node.properties.result || 'result'
                }
            };
        case 'parallel':
            return {
                parallel: [] // 简化结构，直接数组不需要branches包装
            };
        // 其他节点类型...
        default:
            return { activity: { name: 'Placeholder', result: 'result' } };
    }
}

function parseJSONSafely(jsonString) {
    if (!jsonString || !jsonString.trim()) return null;
    try {
        return JSON.parse(jsonString);
    } catch (e) {
        console.warn('Invalid JSON in args:', jsonString);
        return null;
    }
}

function generateYAMLString(obj) {
    // 简化的YAML序列化
    function yamlify(value, indent = 0) {
        const spaces = '  '.repeat(indent);
        
        if (value === null || value === undefined) {
            return 'null';
        }
        
        if (typeof value === 'string') {
            // 简单字符串不需要引号，包含特殊字符的需要引号
            if (/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(value) || /^[0-9]+$/.test(value)) {
                return `"${value}"`;
            }
            return `"${value}"`;
        }
        
        if (typeof value === 'number' || typeof value === 'boolean') {
            return String(value);
        }
        
        if (Array.isArray(value)) {
            if (value.length === 0) {
                return '[]';
            }
            let result = '';
            for (let i = 0; i < value.length; i++) {
                result += `\n${spaces}- `;
                const itemYaml = yamlify(value[i], indent + 1);
                if (typeof value[i] === 'object' && value[i] !== null && !Array.isArray(value[i])) {
                    // 对象项需要特殊处理
                    const lines = itemYaml.split('\n').filter(line => line.trim());
                    result += lines[0];
                    for (let j = 1; j < lines.length; j++) {
                        result += `\n${spaces}  ${lines[j]}`;
                    }
                } else {
                    result += itemYaml;
                }
            }
            return result;
        }
        
        if (typeof value === 'object') {
            let result = '';
            const keys = Object.keys(value);
            for (let i = 0; i < keys.length; i++) {
                const key = keys[i];
                const val = value[key];
                if (i > 0) result += '\n';
                result += `${spaces}${key}: `;
                
                const valYaml = yamlify(val, indent + 1);
                if (typeof val === 'object' && val !== null) {
                    if (Array.isArray(val) && val.length > 0) {
                        result += valYaml;
                    } else if (!Array.isArray(val)) {
                        result += `\n${valYaml}`;
                    } else {
                        result += valYaml;
                    }
                } else {
                    result += valYaml;
                }
            }
            return result;
        }
        
        return String(value);
    }
    
    return yamlify(obj).trim();
}

function validateWorkflow() {
    // 从可编辑的YAML编辑器读取内容，如果为空则先生成
    let yamlContent = document.getElementById('yamlEditor').value;
    
    if (!yamlContent.trim()) {
        console.log("YAML editor is empty, generating YAML first");
        generateYAML();
        yamlContent = document.getElementById('yamlEditor').value;
    }
    
    console.log("Validating YAML:", yamlContent.substring(0, 100) + "...");
    
    fetch('/api/workflow/execute', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ yaml: yamlContent })
    })
    .then(response => response.json())
    .then(data => {
        const validationResults = document.getElementById('validationResults');
        
        if (data.success) {
            validationResults.innerHTML = `
                <div style="color: #4CAF50;">
                    <i class="fas fa-check-circle"></i>
                    <strong>Validation Successful</strong>
                    <p>Workflow structure is valid and ready for execution.</p>
                </div>
            `;
            updateStatus('Workflow validation passed');
        } else {
            validationResults.innerHTML = `
                <div style="color: #f44336;">
                    <i class="fas fa-times-circle"></i>
                    <strong>Validation Failed</strong>
                    <p>${data.error}</p>
                </div>
            `;
            updateStatus('Workflow validation failed');
        }
        
        switchTab('validation');
        toggleResultsPanel(true);
    })
    .catch(error => {
        console.error('Validation error:', error);
        updateStatus('Validation request failed');
    });
}

function executeWorkflow() {
    const yamlContent = document.getElementById('yamlCode').textContent;
    
    if (!yamlContent.trim()) {
        updateStatus('No workflow to execute');
        return;
    }
    
    updateStatus('Executing workflow...');
    
    fetch('/api/workflow/execute', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ yaml: yamlContent })
    })
    .then(response => response.json())
    .then(data => {
        const executionResults = document.getElementById('executionResults');
        
        if (data.success) {
            executionResults.innerHTML = `
                <div style="color: #4CAF50; margin-bottom: 16px;">
                    <h4><i class="fas fa-check-circle"></i> Execution Successful</h4>
                    <p><strong>Workflow ID:</strong> ${data.workflowId}</p>
                    <p><strong>Run ID:</strong> ${data.runId}</p>
                </div>
                <div style="background: #f8f9fa; padding: 16px; border-radius: 8px;">
                    <h5>Results:</h5>
                    <pre>${JSON.stringify(data.result, null, 2)}</pre>
                </div>
            `;
            updateStatus('Workflow executed successfully');
        } else {
            executionResults.innerHTML = `
                <div style="color: #f44336;">
                    <h4><i class="fas fa-times-circle"></i> Execution Failed</h4>
                    <p><strong>Error:</strong> ${data.error}</p>
                </div>
            `;
            updateStatus('Workflow execution failed');
        }
        
        switchTab('execution');
        toggleResultsPanel(true);
    })
    .catch(error => {
        console.error('Execution error:', error);
        updateStatus('Execution request failed');
    });
}

// UI 控制函数
function updateStatus(message) {
    document.querySelector('.status-text').textContent = message;
    console.log('Status:', message);
}

function toggleResultsPanel(show) {
    const container = document.getElementById('resultsContainer');
    const btn = document.getElementById('toggleResults');
    
    if (show === true || (show === undefined && container.style.display === 'none')) {
        container.style.display = 'flex';
        btn.innerHTML = '<i class="fas fa-chevron-down"></i> Hide Results';
    } else {
        container.style.display = 'none';
        btn.innerHTML = '<i class="fas fa-terminal"></i> Results';
    }
}

function toggleYamlPanel() {
    generateYAML();
    switchTab('yaml');
    toggleResultsPanel(true);
}

function switchTab(tabName) {
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.classList.remove('active');
        if (btn.dataset.tab === tabName) {
            btn.classList.add('active');
        }
    });
    
    document.querySelectorAll('.tab-pane').forEach(pane => {
        pane.classList.remove('active');
        if (pane.id === tabName + 'Results' || pane.id === tabName + 'Output' || pane.id === tabName + 'Results') {
            pane.classList.add('active');
        }
    });
}

function loadExamples() {
    fetch('/api/examples')
        .then(response => response.json())
        .then(examples => {
            const select = document.getElementById('exampleSelect');
            select.innerHTML = '<option value="">Load Example...</option>';
            
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
    
    // 这里可以实现从示例YAML反向生成节点图
    updateStatus(`Loading example: ${selectedExample}`);
    
    // 简化版：直接显示YAML
    fetch('/api/examples')
        .then(response => response.json())
        .then(examples => {
            if (examples[selectedExample]) {
                document.getElementById('yamlEditor').value = examples[selectedExample];
                switchTab('yaml');
                toggleResultsPanel(true);
                updateStatus(`Loaded example: ${selectedExample}`);
            }
        });
}

function saveWorkflow() {
    const workflow = generateYAML();
    const blob = new Blob([document.getElementById('yamlCode').textContent], { type: 'text/yaml' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'workflow.yaml';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    updateStatus('Workflow saved');
}

function handleKeyboard(e) {
    if (e.key === 'Delete' && selectedNode) {
        deleteNode(selectedNode);
    }
    
    if (e.ctrlKey || e.metaKey) {
        switch(e.key) {
            case 's':
                e.preventDefault();
                saveWorkflow();
                break;
            case 'Enter':
                e.preventDefault();
                executeWorkflow();
                break;
        }
    }
    
    if (e.key === 'Escape') {
        closeModal();
        hideContextMenu();
    }
}

// 工具函数
function copyNode(nodeId) {
    // 复制节点的简化实现
    const nodeData = workflowData.nodes.get(nodeId);
    if (!nodeData) return;
    
    const newPosition = {
        x: nodeData.position.x + 50,
        y: nodeData.position.y + 50
    };
    
    createNode(nodeData.type, newPosition, { ...nodeData.properties });
    updateStatus('Node copied');
}

// 自动保存和YAML生成
setInterval(() => {
    if (workflowData.nodes.size > 0) {
        generateYAML();
    }
}, 2000);

console.log('Visual Workflow Editor initialized');