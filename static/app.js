// claude-watch frontend

var state = {
  conversations: [],
  projects: [],
  selectedProject: 'all',
  selectedSessionId: null,
  currentSession: null,
  searchQuery: '',
  searchResults: [],
  page: 1,
  totalConversations: 0,
  searchPage: 1,
  searchTotal: 0,
};

var LIMIT = 50;
var searchTimer = null;

// DOM refs
var searchInput = document.getElementById('search-input');
var searchClear = document.getElementById('search-clear');
var projectFilter = document.getElementById('project-filter');
var convList = document.getElementById('conversation-list');
var mainPanel = document.getElementById('main-panel');
var emptyState = document.getElementById('empty-state');
var sessionHeader = document.getElementById('session-header');
var messageThread = document.getElementById('message-thread');
var memoryPanel = document.getElementById('memory-panel');

// Init
function init() {
  fetch('/api/status')
    .then(function(r) { return r.json(); })
    .then(function(data) {
      fetchConversations();
    })
    .catch(function(err) {
      console.error('Status check failed:', err);
      fetchConversations();
    });

  searchInput.addEventListener('input', function() {
    clearTimeout(searchTimer);
    var q = searchInput.value.trim();
    searchClear.hidden = !q;
    if (!q) {
      state.searchQuery = '';
      state.searchResults = [];
      renderSidebar();
      return;
    }
    searchTimer = setTimeout(function() {
      search(q);
    }, 300);
  });

  searchClear.addEventListener('click', function() {
    searchInput.value = '';
    searchClear.hidden = true;
    state.searchQuery = '';
    state.searchResults = [];
    renderSidebar();
  });

  projectFilter.addEventListener('change', function() {
    state.selectedProject = projectFilter.value;
    state.page = 1;
    state.conversations = [];
    fetchConversations();
  });
}

// API helpers
function fetchConversations() {
  var url = '/api/conversations?page=' + state.page + '&limit=' + LIMIT;
  if (state.selectedProject !== 'all') {
    url += '&project=' + encodeURIComponent(state.selectedProject);
  }
  fetch(url)
    .then(function(r) { return r.json(); })
    .then(function(data) {
      if (state.page === 1) {
        state.conversations = data.conversations || [];
      } else {
        state.conversations = state.conversations.concat(data.conversations || []);
      }
      state.totalConversations = data.total || 0;
      buildProjectFilter(state.conversations);
      renderSidebar();
    })
    .catch(function(err) {
      console.error('Failed to fetch conversations:', err);
    });
}

function buildProjectFilter(conversations) {
  var seen = {};
  conversations.forEach(function(c) {
    if (c.projectName) seen[c.projectName] = true;
  });
  // Merge with existing projects
  state.projects.forEach(function(p) { seen[p] = true; });
  state.projects = Object.keys(seen).sort();

  var current = projectFilter.value;
  projectFilter.innerHTML = '<option value="all">All projects</option>';
  state.projects.forEach(function(p) {
    var opt = document.createElement('option');
    opt.value = p;
    opt.textContent = p;
    if (p === current) opt.selected = true;
    projectFilter.appendChild(opt);
  });
}

function loadConversation(sessionId) {
  state.selectedSessionId = sessionId;
  // Update sidebar selection
  var items = convList.querySelectorAll('.conv-item, .search-result');
  items.forEach(function(el) {
    el.classList.toggle('selected', el.dataset.sessionId === sessionId);
  });

  // Show loading state immediately
  emptyState.hidden = true;
  sessionHeader.hidden = true;
  messageThread.hidden = false;
  messageThread.innerHTML = '<div class="empty-state" style="height:100%;color:#555;">Loading...</div>';
  memoryPanel.hidden = true;

  fetch('/api/conversations/' + encodeURIComponent(sessionId))
    .then(function(r) { return r.json(); })
    .then(function(data) {
      state.currentSession = data;
      renderSession(data);
    })
    .catch(function(err) {
      console.error('Failed to load conversation:', err);
      messageThread.innerHTML = '<div class="empty-state" style="height:100%;color:#555;">Failed to load conversation.</div>';
    });
}

function search(q) {
  state.searchQuery = q;
  state.searchPage = 1;
  fetchSearchResults();
}

function fetchSearchResults() {
  var url = '/api/search?q=' + encodeURIComponent(state.searchQuery) + '&page=' + state.searchPage + '&limit=' + LIMIT;
  fetch(url)
    .then(function(r) { return r.json(); })
    .then(function(data) {
      if (state.searchPage === 1) {
        state.searchResults = data.results || [];
      } else {
        state.searchResults = state.searchResults.concat(data.results || []);
      }
      state.searchTotal = data.total || 0;
      renderSidebar();
    })
    .catch(function(err) {
      console.error('Search failed:', err);
    });
}

// Renderers
function renderSidebar() {
  if (state.searchQuery) {
    renderSearchResults();
  } else {
    renderConversationList();
  }
}

function renderConversationList() {
  var html = '';
  state.conversations.forEach(function(c) {
    var selected = c.sessionId === state.selectedSessionId ? ' selected' : '';
    var preview = c.firstMessage || c.slug || '(no message)';
    if (preview.length > 80) preview = preview.substring(0, 80) + '...';
    var date = formatRelativeDate(c.lastActiveAt || c.startedAt);
    var compaction = c.hasCompaction ? '<span class="conv-compaction-dot" title="Has compaction"></span>' : '';

    html += '<div class="conv-item' + selected + '" data-session-id="' + escapeAttr(c.sessionId) + '">'
      + '<div class="conv-project">' + escapeHtml(c.projectName || 'unknown') + '</div>'
      + '<div class="conv-preview">' + escapeHtml(preview) + '</div>'
      + '<div class="conv-meta">'
      + '<span>' + date + '</span>'
      + '<span class="conv-badge">' + (c.messageCount || 0) + ' msgs</span>'
      + compaction
      + '</div>'
      + '</div>';
  });

  if (state.conversations.length < state.totalConversations) {
    html += '<div class="load-more"><button id="load-more-btn">Load more</button></div>';
  }

  convList.innerHTML = html;

  convList.querySelectorAll('.conv-item').forEach(function(el) {
    el.addEventListener('click', function() {
      loadConversation(el.dataset.sessionId);
    });
  });

  var loadMoreBtn = document.getElementById('load-more-btn');
  if (loadMoreBtn) {
    loadMoreBtn.addEventListener('click', function() {
      state.page++;
      fetchConversations();
    });
  }
}

function renderSearchResults() {
  var html = '';
  state.searchResults.forEach(function(r) {
    var date = formatRelativeDate(r.timestamp);
    html += '<div class="search-result" data-session-id="' + escapeAttr(r.sessionId) + '" data-uuid="' + escapeAttr(r.uuid || '') + '">'
      + '<div class="sr-project">' + escapeHtml(r.projectName || 'unknown') + '</div>'
      + '<div class="sr-snippet">' + highlightSnippet(r.snippet || r.contentText || '') + '</div>'
      + '<div class="sr-time">' + date + '</div>'
      + '</div>';
  });

  if (state.searchResults.length < state.searchTotal) {
    html += '<div class="load-more"><button id="load-more-search-btn">Load more</button></div>';
  }

  if (!state.searchResults.length) {
    html = '<div class="empty-state" style="padding:20px;color:#555;">No results</div>';
  }

  convList.innerHTML = html;

  convList.querySelectorAll('.search-result').forEach(function(el) {
    el.addEventListener('click', function() {
      var sid = el.dataset.sessionId;
      var uuid = el.dataset.uuid;
      loadConversation(sid);
      // Scroll to message after load
      if (uuid) {
        setTimeout(function() {
          var target = document.querySelector('[data-uuid="' + uuid + '"]');
          if (target) target.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }, 500);
      }
    });
  });

  var loadMoreBtn = document.getElementById('load-more-search-btn');
  if (loadMoreBtn) {
    loadMoreBtn.addEventListener('click', function() {
      state.searchPage++;
      fetchSearchResults();
    });
  }
}

function highlightSnippet(text) {
  // The API might return snippet with <mark> tags already, or we do it client-side
  if (text.indexOf('<mark>') !== -1) return text;
  var escaped = escapeHtml(text);
  if (!state.searchQuery) return escaped;
  // Simple highlight: split query on , and ; and space for highlighting
  var terms = state.searchQuery.replace(/[,;]/g, ' ').split(/\s+/).filter(Boolean);
  terms.forEach(function(term) {
    var re = new RegExp('(' + escapeRegex(term) + ')', 'gi');
    escaped = escaped.replace(re, '<mark>$1</mark>');
  });
  return escaped;
}

function renderSession(data) {
  var session = data.session || {};
  var messages = data.messages || [];
  var memoryMd = data.memoryMd || '';

  emptyState.hidden = true;
  sessionHeader.hidden = false;
  messageThread.hidden = false;

  // Session header
  var headerHtml = '<span class="sh-project">' + escapeHtml(session.projectName || '') + '</span>';
  if (session.slug) {
    headerHtml += '<span class="sh-id">' + escapeHtml(session.slug) + '</span>';
  }
  headerHtml += '<span class="sh-id">' + escapeHtml(session.sessionId || '') + '<button class="sh-copy-btn" id="copy-sid-btn">Copy</button></span>';
  if (session.gitBranch) {
    headerHtml += '<span class="sh-branch">' + escapeHtml(session.gitBranch) + '</span>';
  }
  if (session.model) {
    headerHtml += '<span class="sh-model">' + escapeHtml(session.model) + '</span>';
  }
  sessionHeader.innerHTML = headerHtml;

  document.getElementById('copy-sid-btn').addEventListener('click', function() {
    var btn = this;
    navigator.clipboard.writeText(session.sessionId).then(function() {
      btn.textContent = 'Copied!';
      setTimeout(function() { btn.textContent = 'Copy'; }, 1500);
    });
  });

  // Messages
  renderMessages(messages);

  // Memory
  if (memoryMd) {
    memoryPanel.hidden = false;
    memoryPanel.innerHTML = '<details><summary>MEMORY.md</summary><pre>' + escapeHtml(memoryMd) + '</pre></details>';
  } else {
    memoryPanel.hidden = true;
    memoryPanel.innerHTML = '';
  }
}

function renderMessages(messages) {
  var html = '';
  messages.forEach(function(msg) {
    html += renderMessage(msg);
  });
  messageThread.innerHTML = html;
  messageThread.scrollTop = 0;
}

function renderMessage(msg) {
  var type = msg.msgType || msg.role || 'unknown';

  if (type === 'compact_boundary') {
    return renderCompactBoundary(msg);
  }

  if (type === 'compact_summary') {
    return renderCompactSummary(msg);
  }

  var cssClass = 'msg';
  var roleLabel = '';
  var roleClass = '';

  if (type === 'user') {
    cssClass += ' msg-user';
    roleLabel = 'User';
    roleClass = 'role-user';
  } else if (type === 'assistant') {
    cssClass += ' msg-assistant';
    roleLabel = 'Assistant';
    roleClass = 'role-assistant';
  } else {
    roleLabel = type;
    roleClass = '';
  }

  var ts = msg.timestamp ? formatTimestamp(msg.timestamp) : '';

  // Parse content blocks from contentJson (full fidelity) or fall back to contentText
  var blocks = [];
  if (msg.contentJson) {
    try { blocks = JSON.parse(msg.contentJson); } catch(e) {}
  }

  var textHtml = '';
  var toolHtml = '';

  if (blocks.length > 0) {
    // Render each block by type
    blocks.forEach(function(block) {
      if (block.type === 'text' && block.text) {
        textHtml += '<div class="msg-content">' + renderMarkdown(block.text) + '</div>';
      } else if (block.type === 'tool_use') {
        var inputStr = '';
        try { inputStr = JSON.stringify(block.input, null, 2); } catch(e) { inputStr = String(block.input); }
        toolHtml += '<details class="tool-use">'
          + '<summary>&#x25B6; Tool: <strong>' + escapeHtml(block.name || 'unknown') + '</strong></summary>'
          + '<pre><code>' + escapeHtml(inputStr) + '</code></pre>'
          + '</details>';
      } else if (block.type === 'tool_result') {
        var resultContent = '';
        if (Array.isArray(block.content)) {
          block.content.forEach(function(c) {
            if (c.type === 'text') resultContent += c.text;
          });
        } else if (typeof block.content === 'string') {
          resultContent = block.content;
        }
        if (resultContent) {
          toolHtml += '<details class="tool-result">'
            + '<summary>&#x25C0; Tool result</summary>'
            + '<pre><code>' + escapeHtml(resultContent.substring(0, 4000)) + (resultContent.length > 4000 ? '\n… (truncated)' : '') + '</code></pre>'
            + '</details>';
        }
      }
    });
  } else if (msg.contentText) {
    // Fallback: plain text
    textHtml = '<div class="msg-content">' + renderMarkdown(msg.contentText) + '</div>';
  }

  // If nothing to show, still render the message label
  if (!textHtml && !toolHtml) {
    textHtml = '<div class="msg-content msg-empty">(no text content)</div>';
  }

  return '<div class="msg ' + cssClass + '" data-uuid="' + escapeAttr(msg.uuid || '') + '">'
    + '<div class="msg-label"><span class="role ' + roleClass + '">' + roleLabel + '</span><span class="ts">' + ts + '</span></div>'
    + textHtml
    + toolHtml
    + '</div>';
}

function renderCompactBoundary(msg) {
  var text = msg.contentText || 'Context compacted';
  // Try to parse useful info from contentText
  if (!text || text === '') text = 'Context compacted';
  return '<div class="compact-boundary">' + escapeHtml(text) + '</div>';
}

function renderCompactSummary(msg) {
  var content = msg.contentText || '';
  var ts = msg.timestamp ? formatTimestamp(msg.timestamp) : '';
  return '<div class="msg msg-compact-summary" data-uuid="' + escapeAttr(msg.uuid || '') + '">'
    + '<div class="msg-label"><span class="role role-summary">Summary</span><span class="ts">' + ts + '</span></div>'
    + '<details><summary>Show compaction summary</summary>'
    + '<div class="msg-content">' + renderMarkdown(content) + '</div>'
    + '</details>'
    + '</div>';
}

// Markdown renderer — always applied to all message content
function renderMarkdown(text) {
  if (!text) return '';

  var escaped = escapeHtml(text);

  // Extract fenced code blocks first (protect from further processing)
  var codeBlocks = [];
  escaped = escaped.replace(/```([\w-]*)\n?([\s\S]*?)```/g, function(_, lang, code) {
    var ph = '\x00CB' + codeBlocks.length + '\x00';
    var langClass = lang ? ' class="lang-' + escapeAttr(lang) + '"' : '';
    codeBlocks.push('<pre><code' + langClass + '>' + code.replace(/\n$/, '') + '</code></pre>');
    return ph;
  });

  // Process line by line for block elements
  var lines = escaped.split('\n');
  var out = [];
  var listStack = []; // stack of {type, indent}

  function closeLists() {
    while (listStack.length) {
      out.push(listStack.pop().type === 'ul' ? '</ul>' : '</ol>');
    }
  }

  for (var i = 0; i < lines.length; i++) {
    var line = lines[i];
    var trimmed = line.trim();

    // Code block placeholder
    if (/^\x00CB\d+\x00$/.test(trimmed)) {
      closeLists();
      out.push(trimmed);
      continue;
    }

    // Heading: # ## ### etc
    var hm = trimmed.match(/^(#{1,6})\s+(.+)$/);
    if (hm) {
      closeLists();
      var lvl = hm[1].length;
      out.push('<h' + lvl + '>' + inlineMarkdown(hm[2]) + '</h' + lvl + '>');
      continue;
    }

    // Horizontal rule
    if (/^[-*_]{3,}$/.test(trimmed)) {
      closeLists();
      out.push('<hr>');
      continue;
    }

    // Blockquote
    var bqm = line.match(/^>\s?(.*)$/);
    if (bqm) {
      closeLists();
      out.push('<blockquote>' + inlineMarkdown(bqm[1]) + '</blockquote>');
      continue;
    }

    // Unordered list item
    var ulm = line.match(/^(\s*)[-*+]\s+(.+)$/);
    if (ulm) {
      var indent = ulm[1].length;
      if (!listStack.length || listStack[listStack.length - 1].type !== 'ul') {
        if (listStack.length && listStack[listStack.length - 1].type !== 'ul') closeLists();
        out.push('<ul>');
        listStack.push({type: 'ul', indent: indent});
      }
      out.push('<li>' + inlineMarkdown(ulm[2]) + '</li>');
      continue;
    }

    // Ordered list item
    var olm = line.match(/^(\s*)\d+[.)]\s+(.+)$/);
    if (olm) {
      var oindent = olm[1].length;
      if (!listStack.length || listStack[listStack.length - 1].type !== 'ol') {
        if (listStack.length && listStack[listStack.length - 1].type !== 'ol') closeLists();
        out.push('<ol>');
        listStack.push({type: 'ol', indent: oindent});
      }
      out.push('<li>' + inlineMarkdown(olm[2]) + '</li>');
      continue;
    }

    // Close lists on blank line or non-list content
    if (trimmed === '') {
      if (listStack.length) closeLists();
      out.push('<br>');
      continue;
    }

    // Close lists when we hit non-list content
    if (listStack.length) closeLists();

    out.push(inlineMarkdown(line) + '<br>');
  }

  closeLists();

  var html = out.join('\n');

  // Restore code blocks
  codeBlocks.forEach(function(block, idx) {
    html = html.replace('\x00CB' + idx + '\x00', block);
  });

  return html;
}

// Inline markdown: bold, italic, inline code, links, strikethrough
function inlineMarkdown(text) {
  if (!text) return '';
  // Inline code (protect first)
  var inlines = [];
  text = text.replace(/`([^`]+)`/g, function(_, code) {
    var ph = '\x00IC' + inlines.length + '\x00';
    inlines.push('<code>' + code + '</code>');
    return ph;
  });
  // Bold+italic
  text = text.replace(/\*\*\*([^*]+)\*\*\*/g, '<strong><em>$1</em></strong>');
  // Bold
  text = text.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  text = text.replace(/__([^_]+)__/g, '<strong>$1</strong>');
  // Italic
  text = text.replace(/\*([^*\n]+)\*/g, '<em>$1</em>');
  text = text.replace(/_([^_\n]+)_/g, '<em>$1</em>');
  // Strikethrough
  text = text.replace(/~~([^~]+)~~/g, '<del>$1</del>');
  // Links — render as non-clickable span (read-only app)
  text = text.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<span class="md-link" title="$2">$1</span>');
  // Restore inline code
  inlines.forEach(function(val, idx) {
    text = text.replace('\x00IC' + idx + '\x00', val);
  });
  return text;
}

// Date formatting
function formatRelativeDate(isoString) {
  if (!isoString) return '';
  var date = new Date(isoString);
  var now = new Date();
  var diffMs = now - date;
  var diffSec = Math.floor(diffMs / 1000);
  var diffMin = Math.floor(diffSec / 60);
  var diffHr = Math.floor(diffMin / 60);
  var diffDay = Math.floor(diffHr / 24);

  if (diffMin < 1) return 'just now';
  if (diffMin < 60) return diffMin + 'm ago';
  if (diffHr < 24) return diffHr + 'h ago';
  if (diffDay === 1) return 'yesterday';
  if (diffDay < 7) return diffDay + 'd ago';

  var months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
  return months[date.getMonth()] + ' ' + date.getDate();
}

function formatTimestamp(isoString) {
  if (!isoString) return '';
  var d = new Date(isoString);
  var h = d.getHours();
  var m = d.getMinutes();
  var s = d.getSeconds();
  return d.getFullYear() + '-' + pad2(d.getMonth() + 1) + '-' + pad2(d.getDate())
    + ' ' + pad2(h) + ':' + pad2(m) + ':' + pad2(s);
}

function pad2(n) { return n < 10 ? '0' + n : '' + n; }

// Escape helpers
function escapeHtml(str) {
  if (!str) return '';
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function escapeAttr(str) {
  if (!str) return '';
  return str.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function escapeRegex(str) {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// Boot
init();
