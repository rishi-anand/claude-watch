// claude-watch frontend

var PROJECT_COLORS = {
  'claude-watch':        { bg: '#f5eddb', text: '#7a4f1a', dot: '#b07830' },
  'palette-agentic-cli': { bg: '#dceee6', text: '#2d5c44', dot: '#4a9070' },
  'teams':               { bg: '#e8e4f4', text: '#3d2870', dot: '#6b52b0' },
  'spectre':             { bg: '#daeef2', text: '#1a4d5c', dot: '#3a8098' },
  'teams-bdd':           { bg: '#f2dde0', text: '#6b2030', dot: '#a84050' },
  'rishi':               { bg: '#dce8f5', text: '#1c3a6a', dot: '#3a6aaa' },
  'spectre-tui':         { bg: '#daeef2', text: '#1a4d5c', dot: '#3a8098' },
  'stylus':              { bg: '#f2e6d8', text: '#6a2c10', dot: '#b05030' },
  'vmo-manager':         { bg: '#e4f0d8', text: '#2e4e1a', dot: '#5a8a30' },
};
var DEFAULT_PROJECT_COLOR = { bg: '#e8e8ea', text: '#4a4a54', dot: '#7a7a88' };

function getProjectColor(name) {
  return PROJECT_COLORS[name] || DEFAULT_PROJECT_COLOR;
}

var state = {
  conversations: [],
  projects: [],
  selectedProject: 'all',
  selectedSessionId: null,
  currentSession: null,
  searchQuery: '',
  searchResults: [],
  searching: false,
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
var searchSpinner = document.getElementById('search-spinner');
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
    searchSpinner.hidden = true;
    state.searchQuery = '';
    state.searchResults = [];
    state.searching = false;
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

function loadConversation(sessionId, scrollToUuid) {
  state.selectedSessionId = sessionId;
  var items = convList.querySelectorAll('.conv-item, .search-result');
  items.forEach(function(el) {
    el.classList.toggle('selected', el.dataset.sessionId === sessionId);
  });

  emptyState.hidden = true;
  sessionHeader.hidden = true;
  messageThread.hidden = false;
  messageThread.innerHTML = '<div class="empty-state" style="height:100%;">Loading…</div>';
  memoryPanel.hidden = true;

  fetch('/api/conversations/' + encodeURIComponent(sessionId))
    .then(function(r) { return r.json(); })
    .then(function(data) {
      state.currentSession = data;
      renderSession(data, scrollToUuid);
    })
    .catch(function(err) {
      console.error('Failed to load conversation:', err);
      messageThread.innerHTML = '<div class="empty-state" style="height:100%;">Failed to load.</div>';
    });
}

function search(q) {
  state.searchQuery = q;
  state.searchPage = 1;
  state.searchResults = [];
  state.searching = true;
  renderSidebar(); // show loading state immediately
  fetchSearchResults();
}

function fetchSearchResults() {
  var url = '/api/search?q=' + encodeURIComponent(state.searchQuery) + '&page=' + state.searchPage + '&limit=' + LIMIT;
  searchSpinner.hidden = false;
  searchClear.hidden = true;
  fetch(url)
    .then(function(r) { return r.json(); })
    .then(function(data) {
      searchSpinner.hidden = true;
      searchClear.hidden = !state.searchQuery;
      state.searching = false;
      if (state.searchPage === 1) {
        state.searchResults = data.results || [];
      } else {
        state.searchResults = state.searchResults.concat(data.results || []);
      }
      state.searchTotal = data.total || 0;
      renderSidebar();
    })
    .catch(function(err) {
      searchSpinner.hidden = true;
      searchClear.hidden = !state.searchQuery;
      state.searching = false;
      renderSidebar();
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
    var color = getProjectColor(c.projectName || '');
    var badge = '<span class="project-badge" style="background:' + color.bg + ';color:' + color.text + '">'
      + escapeHtml(c.projectName || 'unknown') + '</span>';
    var borderStyle = selected ? ' style="border-left-color:' + color.dot + '"' : '';

    html += '<div class="conv-item' + selected + '"' + borderStyle + ' data-session-id="' + escapeAttr(c.sessionId) + '">'
      + '<div class="conv-project">' + badge + '</div>'
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
    var rcolor = getProjectColor(r.projectName || '');
    html += '<div class="search-result" data-session-id="' + escapeAttr(r.sessionId) + '" data-uuid="' + escapeAttr(r.uuid || '') + '">'
      + '<div class="sr-project"><span class="project-badge" style="background:' + rcolor.bg + ';color:' + rcolor.text + '">' + escapeHtml(r.projectName || 'unknown') + '</span></div>'
      + '<div class="sr-snippet">' + highlightSnippet(r.snippet || r.contentText || '') + '</div>'
      + '<div class="sr-time">' + date + '</div>'
      + '</div>';
  });

  if (state.searchResults.length < state.searchTotal) {
    html += '<div class="load-more"><button id="load-more-search-btn">Load more</button></div>';
  }

  if (state.searching) {
    html = '<div class="sidebar-loading"><div class="sidebar-spinner"></div></div>';
  } else if (!state.searchResults.length) {
    html = '<div class="empty-state" style="padding:20px;">No results</div>';
  }

  convList.innerHTML = html;

  convList.querySelectorAll('.search-result').forEach(function(el) {
    el.addEventListener('click', function() {
      loadConversation(el.dataset.sessionId, el.dataset.uuid);
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
  if (!text) return '';
  // Escape all HTML, then restore only <mark>/<\/mark> that the backend inserted.
  // This prevents raw < > from message content breaking the DOM.
  var parts = text.split(/(<mark>|<\/mark>)/);
  return parts.map(function(p) {
    if (p === '<mark>' || p === '</mark>') return p;
    return escapeHtml(p);
  }).join('');
}

function renderSession(data, scrollToUuid) {
  var session = data.session || {};
  var messages = data.messages || [];
  var memoryMd = data.memoryMd || '';

  emptyState.hidden = true;
  sessionHeader.hidden = false;
  messageThread.hidden = false;

  // Session header
  var hcolor = getProjectColor(session.projectName || '');
  var headerHtml = '<span class="project-badge" style="background:' + hcolor.bg + ';color:' + hcolor.text + '">'
    + escapeHtml(session.projectName || 'unknown') + '</span>';
  if (session.slug) {
    headerHtml += '<span class="sh-id">' + escapeHtml(session.slug) + '</span>';
  }
  headerHtml += '<span class="sh-id">' + escapeHtml(session.sessionId || '')
    + '<button class="sh-copy-btn" id="copy-sid-btn">Copy ID</button></span>';
  if (session.gitBranch) {
    headerHtml += '<span class="sh-branch">' + escapeHtml(session.gitBranch) + '</span>';
  }
  if (session.model) {
    headerHtml += '<span class="sh-model">' + escapeHtml(session.model) + '</span>';
  }
  if (session.messageCount) {
    headerHtml += '<span class="sh-msg-count">' + session.messageCount + ' msgs</span>';
  }
  sessionHeader.innerHTML = headerHtml;

  document.getElementById('copy-sid-btn').addEventListener('click', function() {
    var btn = this;
    navigator.clipboard.writeText(session.sessionId).then(function() {
      btn.textContent = '✓ Copied';
      btn.classList.add('copied');
      setTimeout(function() {
        btn.textContent = 'Copy ID';
        btn.classList.remove('copied');
      }, 1500);
    });
  });

  // Messages
  renderMessages(messages, scrollToUuid);

  // Memory
  if (memoryMd) {
    memoryPanel.hidden = false;
    memoryPanel.innerHTML = '<details><summary>MEMORY.md</summary><pre>' + escapeHtml(memoryMd) + '</pre></details>';
  } else {
    memoryPanel.hidden = true;
    memoryPanel.innerHTML = '';
  }
}

// Messages that are always visible and break up work segments.
function isAnchorMessage(msg) {
  var type = msg.msgType || msg.role || '';
  if (type === 'compact_boundary' || type === 'compact_summary') return true;
  if (type !== 'user') return false;
  // User message must have actual typed text (not just tool results)
  var blocks = [];
  if (msg.contentJson) { try { blocks = JSON.parse(msg.contentJson); } catch(e) {} }
  if (blocks.length === 0) return !!msg.contentText;
  return blocks.some(function(b) { return b.type === 'text' && b.text; });
}

function renderMessages(messages, scrollToUuid) {
  // Split messages into anchor messages (user text, compaction markers) and
  // work segments (everything in between — tool calls, tool results, internal turns).
  var segments = [];
  var work = [];

  messages.forEach(function(msg) {
    if (isAnchorMessage(msg)) {
      if (work.length) { segments.push({ type: 'work', messages: work }); work = []; }
      segments.push({ type: 'anchor', msg: msg });
    } else {
      work.push(msg);
    }
  });
  if (work.length) segments.push({ type: 'work', messages: work });

  segments.reverse();

  var html = '';
  segments.forEach(function(seg) {
    if (seg.type === 'anchor') {
      html += renderMessage(seg.msg);
    } else {
      html += renderWorkSegment(seg.messages);
    }
  });

  messageThread.innerHTML = html;

  if (scrollToUuid) {
    var target = messageThread.querySelector('[data-uuid="' + scrollToUuid + '"]');
    if (target) {
      // Open any parent <details> so the target is visible
      var el = target;
      while (el && el !== messageThread) {
        if (el.tagName === 'DETAILS') el.open = true;
        el = el.parentElement;
      }
      target.scrollIntoView({ block: 'center' });
      target.classList.add('msg-highlight');
      setTimeout(function() { target.classList.remove('msg-highlight'); }, 2000);
      return;
    }
  }
  messageThread.scrollTop = 0;
}

function renderWorkSegment(messages) {
  var toolCallCount = 0;
  var msgHtml = '';
  messages.forEach(function(msg) {
    msgHtml += renderMessage(msg);
    var blocks = [];
    if (msg.contentJson) { try { blocks = JSON.parse(msg.contentJson); } catch(e) {} }
    blocks.forEach(function(b) { if (b.type === 'tool_use') toolCallCount++; });
  });

  var steps = messages.length;
  var label = steps + ' step' + (steps !== 1 ? 's' : '');
  if (toolCallCount > 0) label += ' &middot; ' + toolCallCount + ' tool call' + (toolCallCount !== 1 ? 's' : '');

  return '<details class="work-segment">'
    + '<summary><span class="work-segment-badge">&#x2699; ' + label + '</span></summary>'
    + '<div class="work-segment-body">' + msgHtml + '</div>'
    + '</details>';
}

function renderMessage(msg) {
  var type = msg.msgType || msg.role || 'unknown';

  if (type === 'compact_boundary') {
    return renderCompactBoundary(msg);
  }

  if (type === 'compact_summary') {
    return renderCompactSummary(msg);
  }

  var roleLabel = '';
  var roleClass = '';
  var avatarClass = '';
  var avatarLetter = '';

  var msgClass = 'msg';
  if (type === 'user') {
    msgClass += ' msg-user';
    roleLabel = 'User';
    roleClass = 'role-user';
    avatarClass = 'user-avatar';
    avatarLetter = 'U';
  } else if (type === 'assistant') {
    msgClass += ' msg-assistant';
    roleLabel = 'Assistant';
    roleClass = 'role-assistant';
    avatarClass = 'asst-avatar';
    avatarLetter = 'A';
  } else {
    roleLabel = type;
    avatarClass = 'asst-avatar';
    avatarLetter = type[0] ? type[0].toUpperCase() : '?';
  }

  var ts = msg.timestamp ? formatTimestamp(msg.timestamp) : '';

  // Parse content blocks
  var blocks = [];
  if (msg.contentJson) {
    try { blocks = JSON.parse(msg.contentJson); } catch(e) {}
  }

  // Categorise blocks
  var textBlocks = [];
  var toolUseBlocks = [];
  var toolResultBlocks = [];
  blocks.forEach(function(b) {
    if (b.type === 'text' && b.text)  textBlocks.push(b);
    else if (b.type === 'tool_use')   toolUseBlocks.push(b);
    else if (b.type === 'tool_result') toolResultBlocks.push(b);
  });

  // User messages that contain ONLY tool results are tool-exchange plumbing,
  // not something the user typed. Render them as a small collapsible row.
  if (type === 'user' && blocks.length > 0 && textBlocks.length === 0 && toolResultBlocks.length > 0) {
    return renderToolResultRow(msg, toolResultBlocks, ts);
  }

  var textHtml = '';
  var toolHtml = '';

  if (blocks.length > 0) {
    textBlocks.forEach(function(b) {
      textHtml += '<div class="msg-content">' + renderMarkdown(b.text) + '</div>';
    });
    toolUseBlocks.forEach(function(b) {
      var inputStr = '';
      try { inputStr = JSON.stringify(b.input, null, 2); } catch(e) { inputStr = String(b.input); }
      toolHtml += '<details class="tool-use">'
        + '<summary>&#x25B6; Tool: <strong>' + escapeHtml(b.name || 'unknown') + '</strong></summary>'
        + '<pre><code>' + escapeHtml(inputStr) + '</code></pre>'
        + '</details>';
    });
    toolResultBlocks.forEach(function(b) {
      var rc = '';
      if (Array.isArray(b.content)) b.content.forEach(function(c) { if (c.type === 'text') rc += c.text; });
      else if (typeof b.content === 'string') rc = b.content;
      if (rc) {
        toolHtml += '<details class="tool-result">'
          + '<summary>&#x25C0; Tool result</summary>'
          + '<pre><code>' + escapeHtml(rc.substring(0, 4000)) + (rc.length > 4000 ? '\n… (truncated)' : '') + '</code></pre>'
          + '</details>';
      }
    });
  } else if (msg.contentText) {
    textHtml = '<div class="msg-content">' + renderMarkdown(msg.contentText) + '</div>';
  }

  if (!textHtml && !toolHtml) {
    textHtml = '<div class="msg-content msg-empty">(no text content)</div>';
  }

  // For assistant messages: wrap tool calls in a single collapsible section
  var toolSection = '';
  if (type === 'assistant' && toolHtml) {
    var n = toolUseBlocks.length;
    var label = n > 0 ? n + ' tool call' + (n !== 1 ? 's' : '') : 'tool details';
    toolSection = '<details class="tool-calls-section">'
      + '<summary>' + label + '</summary>'
      + '<div class="tool-calls-body">' + toolHtml + '</div>'
      + '</details>';
    toolHtml = '';
  }

  return '<div class="' + msgClass + '" data-uuid="' + escapeAttr(msg.uuid || '') + '">'
    + '<div class="msg-avatar ' + avatarClass + '">' + avatarLetter + '</div>'
    + '<div class="msg-body">'
    + '<div class="msg-label"><span class="role ' + roleClass + '">' + roleLabel + '</span><span class="ts">' + ts + '</span></div>'
    + textHtml
    + toolSection
    + toolHtml
    + '</div>'
    + '</div>';
}

function renderToolResultRow(msg, blocks, ts) {
  var n = blocks.length;
  var contentHtml = '';
  blocks.forEach(function(b) {
    var rc = '';
    if (Array.isArray(b.content)) b.content.forEach(function(c) { if (c.type === 'text') rc += c.text; });
    else if (typeof b.content === 'string') rc = b.content;
    if (rc) {
      contentHtml += '<pre><code>' + escapeHtml(rc.substring(0, 4000))
        + (rc.length > 4000 ? '\n… (truncated)' : '') + '</code></pre>';
    }
  });
  return '<details class="tool-result-row" data-uuid="' + escapeAttr(msg.uuid || '') + '">'
    + '<summary>'
    + '<span class="tool-result-label">&#x25B6; ' + n + ' tool result' + (n !== 1 ? 's' : '') + '</span>'
    + '<span class="ts">' + ts + '</span>'
    + '</summary>'
    + '<div class="tool-result-row-body">' + contentHtml + '</div>'
    + '</details>';
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
  return '<div class="msg" data-uuid="' + escapeAttr(msg.uuid || '') + '">'
    + '<div class="msg-avatar summary-avatar">S</div>'
    + '<div class="msg-body">'
    + '<div class="msg-label"><span class="role role-summary">Summary</span><span class="ts">' + ts + '</span></div>'
    + '<details><summary style="cursor:pointer;color:var(--text-muted);font-size:12px;font-family:var(--font-mono)">Show compaction summary</summary>'
    + '<div class="msg-content">' + renderMarkdown(content) + '</div>'
    + '</details>'
    + '</div>'
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

// ── Theme toggle ──
function initTheme() {
  var btn = document.getElementById('theme-toggle');
  var html = document.documentElement;

  function applyTheme(theme) {
    html.setAttribute('data-theme', theme);
    btn.textContent = theme === 'dark' ? '☀' : '☾';
    btn.title = theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode';
    localStorage.setItem('theme', theme);
  }

  // Apply saved theme (already set before paint by inline script, just sync the button)
  applyTheme(html.getAttribute('data-theme') || 'light');

  btn.addEventListener('click', function() {
    applyTheme(html.getAttribute('data-theme') === 'dark' ? 'light' : 'dark');
  });
}

// ── Resize handle ──
function initResize() {
  var handle = document.getElementById('resize-handle');
  var appEl  = document.getElementById('app');
  var MIN_W  = 160;
  var MAX_W  = 700;

  // Restore saved width
  var saved = parseInt(localStorage.getItem('sidebarWidth'), 10);
  if (saved >= MIN_W && saved <= MAX_W) {
    appEl.style.gridTemplateColumns = saved + 'px 5px 1fr';
  }

  var dragging = false;
  var startX   = 0;
  var startW   = 0;

  handle.addEventListener('mousedown', function(e) {
    dragging = true;
    startX   = e.clientX;
    startW   = parseInt(getComputedStyle(appEl).gridTemplateColumns, 10);
    handle.classList.add('dragging');
    document.body.style.cursor     = 'col-resize';
    document.body.style.userSelect = 'none';
    e.preventDefault();
  });

  document.addEventListener('mousemove', function(e) {
    if (!dragging) return;
    var w = Math.max(MIN_W, Math.min(MAX_W, startW + (e.clientX - startX)));
    appEl.style.gridTemplateColumns = w + 'px 5px 1fr';
  });

  document.addEventListener('mouseup', function() {
    if (!dragging) return;
    dragging = false;
    handle.classList.remove('dragging');
    document.body.style.cursor     = '';
    document.body.style.userSelect = '';
    var w = parseInt(getComputedStyle(appEl).gridTemplateColumns, 10);
    localStorage.setItem('sidebarWidth', w);
  });
}

// Boot
initTheme();
initResize();
init();
