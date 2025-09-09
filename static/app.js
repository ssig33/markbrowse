// Markdown rendering script
document.addEventListener('DOMContentLoaded', function() {
    // Get markdown content from the pre element
    const markdownElement = document.getElementById('markdown-content');
    const renderedElement = document.getElementById('rendered-content');
    
    if (!markdownElement || !renderedElement) {
        console.error('Required elements not found');
        return;
    }
    
    const markdownContent = markdownElement.textContent;
    
    // Configure marked options
    marked.setOptions({
        breaks: true,
        gfm: true,
        headerIds: true,
        highlight: function(code, lang) {
            // Basic syntax highlighting placeholder
            // Could be enhanced with a library like Prism.js or highlight.js
            return code;
        }
    });
    
    // Render markdown to HTML
    let htmlContent = marked.parse(markdownContent);
    
    // Set the rendered content
    renderedElement.innerHTML = htmlContent;
    
    // Initialize Mermaid diagrams
    initializeMermaid();
    
    // Highlight current file in sidebar
    highlightCurrentFile();
});

function initializeMermaid() {
    // Initialize mermaid with custom configuration
    mermaid.initialize({
        startOnLoad: false,
        theme: 'default',
        securityLevel: 'loose',
        flowchart: {
            useMaxWidth: true,
            htmlLabels: true,
            curve: 'basis'
        }
    });
    
    // Find all code blocks with mermaid language
    const mermaidBlocks = document.querySelectorAll('pre code.language-mermaid');
    
    mermaidBlocks.forEach((block, index) => {
        // Get the mermaid code
        const mermaidCode = block.textContent;
        
        // Create a div for the mermaid diagram
        const mermaidDiv = document.createElement('div');
        mermaidDiv.className = 'mermaid';
        mermaidDiv.id = `mermaid-${index}`;
        
        // Replace the code block with the mermaid div
        const pre = block.parentElement;
        pre.parentElement.replaceChild(mermaidDiv, pre);
        
        // Render the mermaid diagram
        try {
            mermaid.render(`mermaid-svg-${index}`, mermaidCode).then(result => {
                mermaidDiv.innerHTML = result.svg;
            }).catch(err => {
                console.error('Mermaid rendering error:', err);
                mermaidDiv.innerHTML = `<pre class="mermaid-error">Error rendering Mermaid diagram:\n${err.message}</pre>`;
            });
        } catch (err) {
            console.error('Mermaid initialization error:', err);
            mermaidDiv.innerHTML = `<pre class="mermaid-error">Error initializing Mermaid:\n${err.message}</pre>`;
        }
    });
}

function highlightCurrentFile() {
    // Get current path from URL
    const currentPath = window.location.pathname.replace(/^\//, '');
    
    // Find all file links in sidebar
    const fileLinks = document.querySelectorAll('.file a');
    
    fileLinks.forEach(link => {
        const linkPath = link.getAttribute('href').replace(/^\//, '');
        if (linkPath === currentPath) {
            link.classList.add('current');
            
            // Expand parent directories
            let parent = link.closest('details');
            while (parent) {
                parent.setAttribute('open', 'open');
                parent = parent.parentElement.closest('details');
            }
        }
    });
}