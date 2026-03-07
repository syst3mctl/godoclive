// GoDoc Live — UI Application
// Expects window.API_DATA or uses sample data for development.

(function () {
  'use strict';

  // ============================================================
  // Sample data — rich enough to exercise all card features
  // ============================================================
  var SAMPLE_DATA = {
    projectName: 'Pet Store API',
    version: 'v2.1.0',
    baseUrl: 'http://localhost:8080',
    endpoints: [
      {
        method: 'POST',
        path: '/auth/login',
        summary: 'Create a new session and return a JWT token',
        tag: 'auth',
        handlerName: 'LoginHandler',
        deprecated: false,
        auth: { required: false, schemes: [] },
        params: [],
        headers: [],
        body: {
          contentType: 'application/json',
          typeName: 'LoginRequest',
          fields: [
            { name: 'email', jsonName: 'email', type: 'string', required: true, example: '"user@example.com"' },
            { name: 'password', jsonName: 'password', type: 'string', required: true, example: '"********"' },
            { name: 'remember_me', jsonName: 'remember_me', type: 'boolean', required: false, example: 'false' }
          ],
          example: '{\n  "email": "user@example.com",\n  "password": "********",\n  "remember_me": false\n}'
        },
        responses: [
          {
            status: 200,
            description: 'Session created',
            contentType: 'application/json',
            source: 'explicit json.Encode call',
            fields: [
              { name: 'access_token', jsonName: 'access_token', type: 'string', required: true, example: '"eyJhbGci..."' },
              { name: 'expires_in', jsonName: 'expires_in', type: 'integer', required: true, example: '3600' }
            ],
            example: '{\n  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",\n  "expires_in": 3600\n}'
          },
          {
            status: 401,
            description: 'Invalid credentials',
            contentType: 'application/json',
            source: 'inferred',
            fields: [
              { name: 'error', jsonName: 'error', type: 'string', required: true, example: '"invalid_credentials"' }
            ],
            example: '{\n  "error": "invalid_credentials"\n}'
          }
        ],
        unresolved: []
      },
      {
        method: 'GET',
        path: '/users',
        summary: 'List all users with optional pagination',
        tag: 'users',
        handlerName: 'ListUsers',
        deprecated: false,
        auth: { required: true, schemes: ['bearer'] },
        params: [
          { name: 'page', in: 'query', type: 'integer', required: false, default: '1', example: '1' },
          { name: 'limit', in: 'query', type: 'integer', required: false, default: '20', example: '20' },
          { name: 'search', in: 'query', type: 'string', required: false, example: '"john"' }
        ],
        headers: [
          { name: 'X-Tenant-ID', type: 'string', required: true }
        ],
        body: null,
        responses: [
          {
            status: 200,
            description: 'Array of users',
            contentType: 'application/json',
            source: 'explicit json.Encode call',
            fields: null,
            example: '[\n  {\n    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",\n    "email": "user@example.com",\n    "name": "John Doe"\n  }\n]'
          }
        ],
        unresolved: []
      },
      {
        method: 'GET',
        path: '/users/{id}',
        summary: 'Get a user by ID',
        tag: 'users',
        handlerName: 'GetUser',
        deprecated: false,
        auth: { required: true, schemes: ['bearer'] },
        params: [
          { name: 'id', in: 'path', type: 'uuid', required: true, example: 'f47ac10b-58cc-4372-a567-0e02b2c3d479' }
        ],
        headers: [],
        body: null,
        responses: [
          {
            status: 200,
            description: 'User object',
            contentType: 'application/json',
            source: 'explicit c.JSON call',
            fields: [
              { name: 'id', jsonName: 'id', type: 'uuid', required: true, example: '"f47ac10b-..."' },
              { name: 'email', jsonName: 'email', type: 'string', required: true, example: '"user@example.com"' },
              { name: 'name', jsonName: 'name', type: 'string', required: true, example: '"John Doe"' },
              { name: 'role', jsonName: 'role', type: 'string', required: false, example: '"user"' }
            ],
            example: '{\n  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",\n  "email": "user@example.com",\n  "name": "John Doe",\n  "role": "user"\n}'
          },
          {
            status: 404,
            description: 'User not found',
            contentType: 'application/json',
            source: 'respond() helper',
            fields: [
              { name: 'error', jsonName: 'error', type: 'string', required: true, example: '"not_found"' }
            ],
            example: '{\n  "error": "not_found"\n}'
          }
        ],
        unresolved: []
      },
      {
        method: 'POST',
        path: '/users',
        summary: 'Create a new user',
        tag: 'users',
        handlerName: 'CreateUser',
        deprecated: false,
        auth: { required: true, schemes: ['bearer'] },
        params: [],
        headers: [],
        body: {
          contentType: 'application/json',
          typeName: 'CreateUserRequest',
          fields: [
            { name: 'email', jsonName: 'email', type: 'string', required: true, example: '"user@example.com"' },
            { name: 'password', jsonName: 'password', type: 'string', required: true, example: '"securePass123"' },
            { name: 'name', jsonName: 'name', type: 'string', required: true, example: '"John Doe"' },
            { name: 'role', jsonName: 'role', type: 'string', required: false, example: '"user"' }
          ],
          example: '{\n  "email": "user@example.com",\n  "password": "securePass123",\n  "name": "John Doe",\n  "role": "user"\n}'
        },
        responses: [
          {
            status: 201,
            description: 'User created',
            contentType: 'application/json',
            source: 'explicit json.Encode call',
            fields: [
              { name: 'id', jsonName: 'id', type: 'uuid', required: true, example: '"f47ac10b-..."' },
              { name: 'email', jsonName: 'email', type: 'string', required: true, example: '"user@example.com"' }
            ],
            example: '{\n  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",\n  "email": "user@example.com"\n}'
          },
          {
            status: 400,
            description: 'Validation error',
            contentType: 'application/json',
            source: 'inferred',
            fields: null,
            example: '{\n  "error": "validation_failed",\n  "details": ["email is required"]\n}'
          }
        ],
        unresolved: []
      },
      {
        method: 'DELETE',
        path: '/users/{id}',
        summary: 'Delete a user by ID',
        tag: 'users',
        handlerName: 'DeleteUser',
        deprecated: false,
        auth: { required: true, schemes: ['bearer'] },
        params: [
          { name: 'id', in: 'path', type: 'uuid', required: true, example: 'f47ac10b-58cc-4372-a567-0e02b2c3d479' }
        ],
        headers: [],
        body: null,
        responses: [
          { status: 204, description: 'User deleted', contentType: null, source: 'explicit WriteHeader call', fields: null, example: null },
          {
            status: 404,
            description: 'User not found',
            contentType: 'application/json',
            source: 'respond() helper',
            fields: null,
            example: '{\n  "error": "not_found"\n}'
          }
        ],
        unresolved: []
      },
      {
        method: 'GET',
        path: '/products',
        summary: 'List all products',
        tag: 'products',
        handlerName: 'ListProducts',
        deprecated: false,
        auth: { required: false, schemes: [] },
        params: [
          { name: 'category', in: 'query', type: 'string', required: false, example: 'electronics' },
          { name: 'sort', in: 'query', type: 'string', required: false, default: 'name', example: 'name' }
        ],
        headers: [],
        body: null,
        responses: [
          {
            status: 200,
            description: 'Array of products',
            contentType: 'application/json',
            source: 'inferred',
            fields: null,
            example: null
          }
        ],
        unresolved: ['response body type']
      },
      {
        method: 'PATCH',
        path: '/products/{id}',
        summary: 'Partially update a product',
        tag: 'products',
        handlerName: 'UpdateProduct',
        deprecated: false,
        auth: { required: true, schemes: ['bearer'] },
        params: [
          { name: 'id', in: 'path', type: 'uuid', required: true, example: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890' }
        ],
        headers: [],
        body: {
          contentType: 'application/json',
          typeName: 'UpdateProductRequest',
          fields: [
            { name: 'name', jsonName: 'name', type: 'string', required: false, example: '"Updated Widget"' },
            { name: 'price', jsonName: 'price', type: 'number', required: false, example: '29.99' },
            { name: 'active', jsonName: 'active', type: 'boolean', required: false, example: 'true' }
          ],
          example: '{\n  "name": "Updated Widget",\n  "price": 29.99,\n  "active": true\n}'
        },
        responses: [
          {
            status: 200,
            description: 'Product updated',
            contentType: 'application/json',
            source: 'explicit c.JSON call',
            fields: null,
            example: '{\n  "id": "a1b2c3d4-...",\n  "name": "Updated Widget",\n  "price": 29.99\n}'
          },
          {
            status: 404,
            description: 'Product not found',
            contentType: 'application/json',
            source: 'respond() helper',
            fields: null,
            example: '{\n  "error": "not_found"\n}'
          }
        ],
        unresolved: ['request body fields']
      },
      {
        method: 'GET',
        path: '/health',
        summary: 'Health check endpoint',
        tag: 'system',
        handlerName: 'HealthCheck',
        deprecated: false,
        auth: { required: false, schemes: [] },
        params: [],
        headers: [],
        body: null,
        responses: [
          {
            status: 200,
            description: 'OK',
            contentType: 'application/json',
            source: 'explicit json.Encode call',
            fields: [
              { name: 'status', jsonName: 'status', type: 'string', required: true, example: '"ok"' },
              { name: 'uptime', jsonName: 'uptime', type: 'string', required: true, example: '"72h15m"' }
            ],
            example: '{\n  "status": "ok",\n  "uptime": "72h15m"\n}'
          }
        ],
        unresolved: []
      },
      {
        method: 'GET',
        path: '/legacy/users',
        summary: 'Legacy user listing (use /users instead)',
        tag: 'system',
        handlerName: 'LegacyListUsers',
        deprecated: true,
        auth: { required: true, schemes: ['basic'] },
        params: [],
        headers: [],
        body: null,
        responses: [
          { status: 302, description: 'Redirects to /users', contentType: null, source: 'explicit redirect', fields: null, example: null }
        ],
        unresolved: []
      }
    ]
  };

  var data = window.API_DATA || SAMPLE_DATA;

  // ============================================================
  // Global auth state
  // ============================================================
  var globalAuth = {
    bearer: '',
    apikeyHeader: 'X-API-Key',
    apikeyValue: '',
    basicUser: '',
    basicPass: ''
  };

  // Compute which auth schemes are actually used by any endpoint
  var usedSchemes = new Set();
  (data.endpoints || []).forEach(function (ep) {
    if (ep.auth && ep.auth.required && ep.auth.schemes) {
      ep.auth.schemes.forEach(function (s) { usedSchemes.add(s); });
    }
  });

  function hasAuth() {
    return !!(globalAuth.bearer || globalAuth.apikeyValue || globalAuth.basicUser);
  }

  function hasAuthFor(scheme) {
    if (scheme === 'bearer') return !!globalAuth.bearer;
    if (scheme === 'apikey') return !!globalAuth.apikeyValue;
    if (scheme === 'basic') return !!globalAuth.basicUser;
    return false;
  }

  function getAuthType() {
    if (globalAuth.bearer) return 'bearer';
    if (globalAuth.apikeyValue) return 'apikey';
    if (globalAuth.basicUser) return 'basic';
    return null;
  }

  // Base URL — fixed from injected data; not user-editable.
  var baseUrl = data.baseUrl || 'http://localhost:8080';

  // ============================================================
  // Helpers
  // ============================================================
  function esc(s) {
    if (s == null) return '';
    var d = document.createElement('div');
    d.textContent = String(s);
    return d.innerHTML;
  }

  function methodClass(m) {
    return m.toLowerCase().replace(/\s/g, '');
  }

  function badgeText(method) {
    return method === 'DELETE' ? 'DEL' : method;
  }

  function statusClass(code) {
    if (code >= 200 && code < 300) return 's2xx';
    if (code >= 400 && code < 500) return 's4xx';
    return 's5xx';
  }

  function statusTabClass(code) {
    if (code >= 200 && code < 300) return 'resp-2xx';
    if (code >= 400 && code < 500) return 'resp-4xx';
    return 'resp-5xx';
  }

  function statusText(code) {
    var map = {
      200:'OK',201:'Created',204:'No Content',301:'Moved Permanently',302:'Found',
      400:'Bad Request',401:'Unauthorized',403:'Forbidden',404:'Not Found',
      422:'Unprocessable Entity',429:'Too Many Requests',500:'Internal Server Error'
    };
    return map[code] || '';
  }

  // SVG icons
  var ICON_LOCK = '<svg aria-hidden="true" viewBox="0 0 16 16" fill="none"><rect x="3" y="7" width="10" height="7" rx="1.5" stroke="currentColor" stroke-width="1.5"/><path d="M5.5 7V5a2.5 2.5 0 015 0v2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>';
  var ICON_KEY = '<svg aria-hidden="true" viewBox="0 0 16 16" fill="none"><circle cx="5.5" cy="10.5" r="2.5" stroke="currentColor" stroke-width="1.5"/><path d="M7.5 8.5L12 4m0 0v2.5m0-2.5H9.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>';
  var ICON_PERSON = '<svg aria-hidden="true" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="5" r="2.5" stroke="currentColor" stroke-width="1.5"/><path d="M3 14c0-2.76 2.24-5 5-5s5 2.24 5 5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>';
  var ICON_COPY = '<svg aria-hidden="true" viewBox="0 0 16 16" fill="none"><rect x="5" y="5" width="8" height="8" rx="1" stroke="currentColor" stroke-width="1.5"/><path d="M3 11V4a1 1 0 011-1h7" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>';
  var ICON_CHECK = '<svg aria-hidden="true" viewBox="0 0 16 16" fill="none"><path d="M3 8.5l3.5 3.5 6.5-8" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';

  function authIcon(scheme) {
    if (scheme === 'bearer') return ICON_LOCK;
    if (scheme === 'apikey') return ICON_KEY;
    if (scheme === 'basic') return ICON_PERSON;
    return ICON_LOCK;
  }

  // ============================================================
  // Path rendering — {param} in method accent color for cards
  // ============================================================
  function renderSidebarPath(path) {
    return esc(path).replace(/(\{[^}]+\})/g, '<span class="path-param">$1</span>');
  }

  function renderCardPath(path, method) {
    var mc = methodClass(method);
    return esc(path).replace(/(\{[^}]+\})/g, '<span style="color:var(--method-' + mc + ')">$1</span>');
  }

  // ============================================================
  // JSON syntax highlighting — 4-color tokenizer (Section 10.5.2)
  // Keys: --text-primary, strings: --syntax-string (green),
  // numbers/bools: --syntax-literal (blue), null: --syntax-null (red),
  // punctuation: --text-secondary
  // ============================================================
  function highlightJsonStr(raw) {
    if (!raw) return '';
    var result = '';
    var i = 0;
    var len = raw.length;
    while (i < len) {
      var ch = raw[i];
      // Skip whitespace
      if (ch === ' ' || ch === '\n' || ch === '\r' || ch === '\t') {
        result += ch;
        i++;
        continue;
      }
      // String
      if (ch === '"') {
        var str = '"';
        i++;
        while (i < len && raw[i] !== '"') {
          if (raw[i] === '\\') { str += raw[i]; i++; }
          if (i < len) { str += raw[i]; i++; }
        }
        if (i < len) { str += '"'; i++; }
        // Check if this is a key (followed by colon)
        var j = i;
        while (j < len && (raw[j] === ' ' || raw[j] === '\t')) j++;
        if (j < len && raw[j] === ':') {
          result += '<span class="json-key">' + esc(str) + '</span>';
        } else {
          result += '<span class="json-string">' + esc(str) + '</span>';
        }
        continue;
      }
      // Number
      if ((ch >= '0' && ch <= '9') || ch === '-') {
        var num = '';
        while (i < len && (raw[i] >= '0' && raw[i] <= '9' || raw[i] === '.' || raw[i] === '-' || raw[i] === 'e' || raw[i] === 'E' || raw[i] === '+')) {
          num += raw[i]; i++;
        }
        result += '<span class="json-number">' + esc(num) + '</span>';
        continue;
      }
      // true/false/null
      if (raw.substr(i, 4) === 'true') {
        result += '<span class="json-boolean">true</span>';
        i += 4; continue;
      }
      if (raw.substr(i, 5) === 'false') {
        result += '<span class="json-boolean">false</span>';
        i += 5; continue;
      }
      if (raw.substr(i, 4) === 'null') {
        result += '<span class="json-null">null</span>';
        i += 4; continue;
      }
      // Punctuation
      if (ch === '{' || ch === '}' || ch === '[' || ch === ']' || ch === ':' || ch === ',') {
        result += '<span class="json-punct">' + ch + '</span>';
        i++; continue;
      }
      result += esc(ch);
      i++;
    }
    return result;
  }

  // ============================================================
  // curl snippet generation
  // ============================================================
  function buildCurl(ep) {
    var url = baseUrl + ep.path;
    // Replace path params with example values
    (ep.params || []).forEach(function (p) {
      if (p.in === 'path' && p.example) {
        url = url.replace('{' + p.name + '}', p.example);
      }
    });
    // Query params
    var queryParts = [];
    (ep.params || []).forEach(function (p) {
      if (p.in === 'query' && p.example) {
        queryParts.push(encodeURIComponent(p.name) + '=' + encodeURIComponent(p.example.replace(/^"|"$/g, '')));
      }
    });
    if (queryParts.length > 0) url += '?' + queryParts.join('&');

    var lines = ['curl -X ' + ep.method + ' ' + JSON.stringify(url)];

    // Auth headers — use the endpoint's required scheme
    var epScheme = (ep.auth && ep.auth.required && ep.auth.schemes && ep.auth.schemes.length > 0) ? ep.auth.schemes[0] : null;
    if (epScheme === 'bearer' && globalAuth.bearer) {
      lines.push('  -H "Authorization: Bearer ' + globalAuth.bearer + '"');
    } else if (epScheme === 'apikey' && globalAuth.apikeyValue) {
      lines.push('  -H "' + globalAuth.apikeyHeader + ': ' + globalAuth.apikeyValue + '"');
    } else if (epScheme === 'basic' && globalAuth.basicUser) {
      lines.push('  -u "' + globalAuth.basicUser + ':' + globalAuth.basicPass + '"');
    }

    // Custom headers
    (ep.headers || []).forEach(function (h) {
      lines.push('  -H "' + h.name + ': your-value"');
    });

    // Body
    if (ep.body && ep.body.example) {
      lines.push('  -H "Content-Type: ' + ep.body.contentType + '"');
      lines.push("  -d '" + ep.body.example + "'");
    }

    return lines.join(' \\\n');
  }

  // ============================================================
  // Copy to clipboard
  // ============================================================
  function copyToClipboard(text, btnEl) {
    navigator.clipboard.writeText(text).then(function () {
      btnEl.innerHTML = ICON_CHECK;
      btnEl.classList.add('copied');
      setTimeout(function () {
        btnEl.innerHTML = ICON_COPY;
        btnEl.classList.remove('copied');
      }, 1500);
    }).catch(function () {
      // fallback
      var ta = document.createElement('textarea');
      ta.value = text;
      ta.style.cssText = 'position:fixed;left:-9999px';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
      btnEl.innerHTML = ICON_CHECK;
      btnEl.classList.add('copied');
      setTimeout(function () {
        btnEl.innerHTML = ICON_COPY;
        btnEl.classList.remove('copied');
      }, 1500);
    });
  }

  // ============================================================
  // Group endpoints by tag
  // ============================================================
  function groupByTag(endpoints) {
    var groups = {};
    var order = [];
    endpoints.forEach(function (ep) {
      var tag = ep.tag || 'default';
      if (!groups[tag]) {
        groups[tag] = [];
        order.push(tag);
      }
      groups[tag].push(ep);
    });
    return { groups: groups, order: order };
  }

  // ============================================================
  // Collapse state persistence
  // ============================================================
  var COLLAPSE_KEY = 'gdl-collapsed';

  function loadCollapsed() {
    try { return JSON.parse(localStorage.getItem(COLLAPSE_KEY)) || {}; }
    catch (e) { return {}; }
  }

  function saveCollapsed(state) {
    localStorage.setItem(COLLAPSE_KEY, JSON.stringify(state));
  }

  var collapsedState = loadCollapsed();
  var grouped = groupByTag(data.endpoints);
  var sidebarGroupsEl = document.getElementById('sidebar-groups');
  var sidebarFooterEl = document.getElementById('sidebar-footer');
  var contentInner = document.getElementById('content-inner');
  var contentArea = document.getElementById('content-area');

  // ============================================================
  // Build sidebar (ARIA roles)
  // ============================================================
  function buildSidebar() {
    var html = '';
    grouped.order.forEach(function (tag) {
      var eps = grouped.groups[tag];
      var isCollapsed = collapsedState[tag] === true;
      html += '<div class="sidebar-group" data-group="' + esc(tag) + '">';
      html += '<div class="group-header" data-group-toggle="' + esc(tag) + '" role="button" tabindex="0" aria-expanded="' + (!isCollapsed) + '">';
      html += '<span class="group-chevron" aria-hidden="true">' + (isCollapsed ? '&#9656;' : '&#9662;') + '</span>';
      html += '<span class="group-name">' + esc(tag) + '</span>';
      html += '</div>';
      html += '<div class="group-content' + (isCollapsed ? ' collapsed' : '') + '" data-group-content="' + esc(tag) + '">';
      eps.forEach(function (ep, idx) {
        var epId = tag + '-' + idx;
        html += '<div class="endpoint-row" data-method="' + ep.method + '" data-ep-id="' + epId + '" tabindex="0" role="link" aria-label="' + ep.method + ' ' + esc(ep.path) + '">';
        html += '<span class="method-badge ' + methodClass(ep.method) + '">' + badgeText(ep.method) + '</span>';
        html += '<span class="endpoint-path">' + renderSidebarPath(ep.path) + '</span>';
        html += '</div>';
      });
      html += '</div></div>';
    });
    sidebarGroupsEl.innerHTML = html;

    // Footer
    var totalEndpoints = data.endpoints.length;
    var totalGroups = grouped.order.length;
    var unresolvedCount = data.endpoints.filter(function (ep) {
      return ep.unresolved && ep.unresolved.length > 0;
    }).length;

    var footerHtml = '<div class="sidebar-footer-stats">';
    footerHtml += totalEndpoints + ' endpoint' + (totalEndpoints !== 1 ? 's' : '');
    footerHtml += '<span class="dot-sep">&middot;</span>';
    footerHtml += totalGroups + ' group' + (totalGroups !== 1 ? 's' : '');
    footerHtml += '</div>';
    if (unresolvedCount > 0) {
      footerHtml += '<div class="sidebar-footer-warning" id="unresolved-filter" role="button" tabindex="0">';
      footerHtml += '&#9888; ' + unresolvedCount + ' partially resolved';
      footerHtml += '</div>';
    }
    sidebarFooterEl.innerHTML = footerHtml;
  }

  buildSidebar();

  // ============================================================
  // Build endpoint cards — full spec
  // ============================================================
  function buildCards() {
    var html = '';
    grouped.order.forEach(function (tag) {
      var eps = grouped.groups[tag];
      eps.forEach(function (ep, idx) {
        var epId = tag + '-' + idx;
        var mc = methodClass(ep.method);
        html += '<div class="endpoint-card" id="card-' + epId + '" data-ep-id="' + epId + '">';

        // === Card Header ===
        html += '<div class="card-header">';
        html += '<div class="card-header-left">';
        html += '<span class="card-method-badge ' + mc + '">' + ep.method + '</span>';
        if (ep.deprecated) {
          html += '<span class="deprecated-label">Deprecated</span>';
          html += '<span class="card-path deprecated">' + renderCardPath(ep.path, ep.method) + '</span>';
        } else {
          html += '<span class="card-path">' + renderCardPath(ep.path, ep.method) + '</span>';
        }
        html += '</div>';
        html += '<div class="card-header-right">';
        // Auth badge
        if (ep.auth && ep.auth.required && ep.auth.schemes.length > 0) {
          html += '<span class="auth-badge">' + authIcon(ep.auth.schemes[0]) + ' ' + esc(ep.auth.schemes.join(', ')) + '</span>';
        }
        // Partial badge
        if (ep.unresolved && ep.unresolved.length > 0) {
          html += '<button class="partial-badge" data-partial-toggle="' + epId + '" aria-expanded="false" title="Partially resolved">';
          html += '&#9888; partial</button>';
        }
        html += '</div></div>';

        // Summary
        if (ep.summary) {
          html += '<div class="card-summary">' + esc(ep.summary) + '</div>';
        }

        // Unresolved callout (hidden by default)
        if (ep.unresolved && ep.unresolved.length > 0) {
          html += '<div class="unresolved-callout hidden" id="unresolved-' + epId + '">';
          html += '<strong>Unresolved items:</strong><ul>';
          ep.unresolved.forEach(function (item) {
            html += '<li>' + esc(item) + '</li>';
          });
          html += '</ul></div>';
        }

        // === Path Parameters ===
        var pathParams = (ep.params || []).filter(function (p) { return p.in === 'path'; });
        if (pathParams.length > 0) {
          html += buildSection('Path Parameters', null, buildPathParamTable(pathParams));
        }

        // === Query Parameters ===
        var queryParams = (ep.params || []).filter(function (p) { return p.in === 'query'; });
        if (queryParams.length > 0) {
          html += buildSection('Query Parameters', null, buildQueryParamTable(queryParams));
        }

        // === Request Headers ===
        if (ep.headers && ep.headers.length > 0) {
          html += buildSection('Request Headers', null, buildHeaderTable(ep.headers));
        }

        // === Request Body ===
        if (ep.body) {
          var bodyContent = buildBodyTabs(ep.body, epId + '-body');
          html += buildSection('Request Body', ep.body.contentType, bodyContent);
        }

        // === Responses ===
        if (ep.responses && ep.responses.length > 0) {
          html += buildSection('Responses', null, buildResponseTabs(ep.responses, epId));
        }

        // === curl tab (always visible) ===
        html += buildSection('curl', null, buildCodeBlock(buildCurl(ep), epId + '-curl', 'bash'));

        // === Try It Panel ===
        html += buildTryItPanel(ep, epId);

        html += '</div>'; // end card
      });
    });
    contentInner.innerHTML = html;
  }

  // ============================================================
  // Section builder
  // ============================================================
  function buildSection(label, extraLabel, content) {
    var h = '<div class="card-section">';
    h += '<div class="card-section-label">' + esc(label);
    if (extraLabel) {
      h += '<span class="content-type-label">' + esc(extraLabel) + '</span>';
    }
    h += '</div>';
    h += content;
    h += '</div>';
    return h;
  }

  // ============================================================
  // Path param table
  // ============================================================
  function buildPathParamTable(params) {
    var h = '<table class="param-table"><thead><tr>';
    h += '<th>Name</th><th>Type</th><th>Required</th><th>Example</th>';
    h += '</tr></thead><tbody>';
    params.forEach(function (p) {
      h += '<tr>';
      h += '<td><span class="param-name">' + esc(p.name) + '</span></td>';
      h += '<td><span class="param-type">' + esc(p.type) + '</span></td>';
      h += '<td>' + requiredPill(p.required) + '</td>';
      h += '<td><span class="param-example">' + esc(p.example || '---') + '</span></td>';
      h += '</tr>';
    });
    h += '</tbody></table>';
    return h;
  }

  // ============================================================
  // Query param table
  // ============================================================
  function buildQueryParamTable(params) {
    var h = '<table class="param-table"><thead><tr>';
    h += '<th>Name</th><th>Type</th><th>Required</th><th>Default</th><th>Example</th>';
    h += '</tr></thead><tbody>';
    params.forEach(function (p) {
      h += '<tr>';
      h += '<td><span class="param-name">' + esc(p.name) + '</span></td>';
      h += '<td><span class="param-type">' + esc(p.type) + '</span></td>';
      h += '<td>' + requiredPill(p.required) + '</td>';
      h += '<td><span class="param-default">' + esc(p.default || '---') + '</span></td>';
      h += '<td><span class="param-example">' + esc(p.example || '---') + '</span></td>';
      h += '</tr>';
    });
    h += '</tbody></table>';
    return h;
  }

  // ============================================================
  // Header table
  // ============================================================
  function buildHeaderTable(headers) {
    var h = '<table class="param-table"><thead><tr>';
    h += '<th>Name</th><th>Type</th><th>Required</th>';
    h += '</tr></thead><tbody>';
    headers.forEach(function (hd) {
      h += '<tr>';
      h += '<td><span class="param-name">' + esc(hd.name) + '</span></td>';
      h += '<td><span class="param-type">' + esc(hd.type) + '</span></td>';
      h += '<td>' + requiredPill(hd.required) + '</td>';
      h += '</tr>';
    });
    h += '</tbody></table>';
    return h;
  }

  // ============================================================
  // Required pill (dot + text for accessibility)
  // ============================================================
  function requiredPill(isRequired) {
    if (isRequired) {
      return '<span class="required-pill is-required"><span class="required-dot" aria-hidden="true"></span> required</span>';
    }
    return '<span class="required-pill is-optional">optional</span>';
  }

  // ============================================================
  // Body tabs: Schema | Example
  // ============================================================
  function buildBodyTabs(body, prefix) {
    var h = '<div class="tab-group" role="tablist" aria-label="Request body views">';
    h += '<button class="tab-btn active" role="tab" aria-selected="true" data-tab="' + prefix + '-schema" id="tab-' + prefix + '-schema" aria-controls="panel-' + prefix + '-schema">Schema</button>';
    h += '<button class="tab-btn" role="tab" aria-selected="false" data-tab="' + prefix + '-example" id="tab-' + prefix + '-example" aria-controls="panel-' + prefix + '-example">Example</button>';
    h += '</div>';

    // Schema panel
    h += '<div class="tab-panel active" id="panel-' + prefix + '-schema" role="tabpanel" aria-labelledby="tab-' + prefix + '-schema">';
    if (body.fields && body.fields.length > 0) {
      h += buildFieldTable(body.fields);
    } else {
      h += '<div class="no-body-msg">' + esc(body.typeName || 'Unknown type') + '</div>';
    }
    h += '</div>';

    // Example panel
    h += '<div class="tab-panel" id="panel-' + prefix + '-example" role="tabpanel" aria-labelledby="tab-' + prefix + '-example">';
    if (body.example) {
      h += buildCodeBlock(body.example, prefix + '-ex', 'json');
    } else {
      h += '<div class="no-body-msg">No example available</div>';
    }
    h += '</div>';

    return h;
  }

  // ============================================================
  // Field table (schema)
  // ============================================================
  function buildFieldTable(fields) {
    var h = '<table class="param-table"><thead><tr>';
    h += '<th>Field</th><th>Type</th><th>Required</th>';
    h += '</tr></thead><tbody>';
    fields.forEach(function (f) {
      h += '<tr>';
      h += '<td><span class="param-name">' + esc(f.jsonName || f.name) + '</span></td>';
      h += '<td><span class="param-type">' + esc(f.type) + '</span></td>';
      h += '<td>';
      if (f.required) {
        h += '<span class="required-dot" aria-hidden="true"></span>';
        h += '<span class="field-required-text"> required</span>';
      }
      h += '</td>';
      h += '</tr>';
    });
    h += '</tbody></table>';
    return h;
  }

  // ============================================================
  // Response tabs (one per status code)
  // ============================================================
  function buildResponseTabs(responses, epId) {
    // Sort by status code
    var sorted = responses.slice().sort(function (a, b) { return a.status - b.status; });

    var h = '<div class="tab-group" role="tablist" aria-label="Response status codes">';
    sorted.forEach(function (resp, i) {
      var active = i === 0 ? ' active' : '';
      var tabClass = statusTabClass(resp.status);
      var selected = i === 0 ? 'true' : 'false';
      h += '<button class="tab-btn ' + tabClass + active + '" role="tab" aria-selected="' + selected + '" ';
      h += 'data-tab="' + epId + '-resp-' + resp.status + '" ';
      h += 'id="tab-' + epId + '-resp-' + resp.status + '" ';
      h += 'aria-controls="panel-' + epId + '-resp-' + resp.status + '">';
      h += resp.status + ' ' + statusText(resp.status);
      h += '</button>';
    });
    h += '</div>';

    // Tab panels
    sorted.forEach(function (resp, i) {
      var active = i === 0 ? ' active' : '';
      h += '<div class="tab-panel' + active + '" id="panel-' + epId + '-resp-' + resp.status + '" ';
      h += 'role="tabpanel" aria-labelledby="tab-' + epId + '-resp-' + resp.status + '">';

      // Body-less responses
      if (!resp.example && !resp.fields) {
        h += '<div class="no-body-msg">No response body</div>';
      } else {
        // Inner Schema/Example tabs
        var innerPrefix = epId + '-resp-' + resp.status;
        h += '<div class="tab-group" role="tablist" aria-label="Response ' + resp.status + ' views">';
        h += '<button class="tab-btn active" role="tab" aria-selected="true" data-tab="' + innerPrefix + '-schema" id="tab-' + innerPrefix + '-schema" aria-controls="panel-' + innerPrefix + '-schema">Schema</button>';
        h += '<button class="tab-btn" role="tab" aria-selected="false" data-tab="' + innerPrefix + '-example" id="tab-' + innerPrefix + '-example" aria-controls="panel-' + innerPrefix + '-example">Example</button>';
        h += '</div>';

        // Schema
        h += '<div class="tab-panel active" id="panel-' + innerPrefix + '-schema" role="tabpanel" aria-labelledby="tab-' + innerPrefix + '-schema">';
        if (resp.fields && resp.fields.length > 0) {
          h += buildFieldTable(resp.fields);
        } else {
          h += '<div class="no-body-msg">Schema not available</div>';
        }
        h += '</div>';

        // Example
        h += '<div class="tab-panel" id="panel-' + innerPrefix + '-example" role="tabpanel" aria-labelledby="tab-' + innerPrefix + '-example">';
        if (resp.example) {
          h += buildCodeBlock(resp.example, innerPrefix + '-ex', 'json');
        } else {
          h += '<div class="no-body-msg">No example available</div>';
        }
        h += '</div>';
      }

      // Source label
      if (resp.source) {
        h += '<div class="response-source">detected from: ' + esc(resp.source) + '</div>';
      }

      h += '</div>'; // end tab panel
    });

    return h;
  }

  // ============================================================
  // Code block with copy button
  // ============================================================
  function buildCodeBlock(content, id, lang) {
    var highlighted = lang === 'json' ? highlightJsonStr(content) : esc(content);
    var h = '<div class="code-block-wrapper">';
    h += '<button class="copy-btn" data-copy-target="' + id + '" aria-label="Copy to clipboard" title="Copy">' + ICON_COPY + '</button>';
    h += '<pre class="code-block" id="code-' + id + '" data-raw="' + esc(content).replace(/"/g, '&quot;') + '">' + highlighted + '</pre>';
    h += '</div>';
    return h;
  }

  // ============================================================
  // Try It Panel
  // ============================================================
  function buildTryItPanel(ep, epId) {
    var mc = methodClass(ep.method);
    var h = '<button class="try-it-toggle" data-tryit="' + epId + '" aria-expanded="false">';
    h += '<span class="chevron" aria-hidden="true">&#9656;</span> Try It';
    h += '</button>';
    h += '<div class="try-it-panel" id="tryit-' + epId + '" aria-label="Try It panel for ' + ep.method + ' ' + esc(ep.path) + '">';

    // Path with inline inputs
    var pathParams = (ep.params || []).filter(function (p) { return p.in === 'path'; });
    if (pathParams.length > 0) {
      h += '<div class="try-it-row">';
      h += '<span class="try-it-label">Path</span>';
      h += '<div class="try-it-path">';
      var pathParts = ep.path.split(/(\{[^}]+\})/);
      pathParts.forEach(function (part) {
        var match = part.match(/^\{(.+)\}$/);
        if (match) {
          var paramName = match[1];
          var param = pathParams.find(function (p) { return p.name === paramName; });
          var val = param && param.example ? param.example : '';
          h += '<input type="text" class="try-it-path-input" data-param="' + esc(paramName) + '" value="' + esc(val) + '" aria-label="Path parameter ' + esc(paramName) + '">';
        } else if (part) {
          h += '<span>' + esc(part) + '</span>';
        }
      });
      h += '</div></div>';
    }

    // Query params
    var queryParams = (ep.params || []).filter(function (p) { return p.in === 'query'; });
    if (queryParams.length > 0) {
      h += '<div class="try-it-row">';
      h += '<span class="try-it-label">Query</span>';
      h += '<div class="try-it-params">';
      queryParams.forEach(function (p) {
        h += '<div class="try-it-param">';
        h += '<span class="try-it-param-label">';
        if (p.required) h += '<span class="required-indicator" aria-hidden="true">&#8226; </span>';
        h += esc(p.name) + '</span>';
        var placeholder = p.default || p.example || '';
        h += '<input type="text" class="try-it-param-input" data-param="' + esc(p.name) + '" placeholder="' + esc(placeholder) + '" ';
        h += 'value="' + esc(p.example ? p.example.replace(/^"|"$/g, '') : '') + '" ';
        h += 'aria-label="Query parameter ' + esc(p.name) + (p.required ? ' (required)' : '') + '">';
        h += '</div>';
      });
      h += '</div></div>';
    }

    // Headers
    if (ep.headers && ep.headers.length > 0) {
      h += '<div class="try-it-row">';
      h += '<span class="try-it-label">Headers</span>';
      h += '<div class="try-it-params">';
      ep.headers.forEach(function (hd) {
        h += '<div class="try-it-param">';
        h += '<span class="try-it-param-label">' + esc(hd.name) + '</span>';
        h += '<input type="text" class="try-it-param-input" data-header="' + esc(hd.name) + '" placeholder="your-value" aria-label="Header ' + esc(hd.name) + '">';
        h += '</div>';
      });
      h += '</div></div>';
    }

    // Body — show for detected bodies or methods that commonly carry a body
    var bodyMethods = ['POST', 'PUT', 'PATCH'];
    var showBodyArea = ep.body || bodyMethods.indexOf(ep.method) >= 0;
    if (showBodyArea) {
      var bodyExample = (ep.body && ep.body.example) ? ep.body.example : '';
      var ctypeLabel = ep.body ? ep.body.contentType : 'application/json';
      h += '<div class="try-it-row">';
      h += '<span class="try-it-label">Body</span>';
      h += '<div class="try-it-body-wrapper">';
      var bodyPlaceholder = bodyExample ? '' : '{}';
      h += '<textarea class="try-it-body-textarea" aria-label="Request body" placeholder="' + bodyPlaceholder + '">' + esc(bodyExample) + '</textarea>';
      h += '<span class="try-it-body-content-type">' + esc(ctypeLabel) + '</span>';
      h += '</div>';
      h += '</div>';
    }

    // Auth display
    h += '<div class="try-it-row">';
    h += '<span class="try-it-label">Auth</span>';
    h += '<div class="try-it-auth" id="tryit-auth-' + epId + '">';
    h += renderTryItAuth(ep);
    h += '</div></div>';

    // Send button
    h += '<div class="try-it-actions">';
    h += '<button class="send-btn ' + mc + '" data-send="' + epId + '">Send Request</button>';
    h += '</div>';

    // Response area (empty until request fires)
    h += '<div class="try-it-response" id="tryit-resp-' + epId + '" style="display:none"></div>';

    h += '</div>'; // end try-it-panel
    return h;
  }

  function renderTryItAuth(ep) {
    if (ep.auth && ep.auth.required && ep.auth.schemes && ep.auth.schemes.length > 0) {
      var scheme = ep.auth.schemes[0];
      if (hasAuthFor(scheme)) {
        return ICON_LOCK + ' <span>' + scheme + ' configured</span>';
      }
      if (hasAuth()) {
        return '<span class="try-it-auth warning">&#9888; This endpoint requires ' + esc(scheme) + ' auth — configure it in Authorize</span>';
      }
      return '<span class="try-it-auth warning">&#9888; No ' + esc(scheme) + ' auth configured — this request will likely fail</span>';
    }
    return '<span>No auth required</span>';
  }

  // ============================================================
  // Render everything
  // ============================================================
  buildCards();
  document.getElementById('project-name').textContent = data.projectName || 'My API';
  document.getElementById('project-version').textContent = data.version || '';

  // ============================================================
  // Tab switching (event delegation)
  // ============================================================
  contentInner.addEventListener('click', function (e) {
    var tabBtn = e.target.closest('.tab-btn[data-tab]');
    if (!tabBtn) return;
    var tabId = tabBtn.getAttribute('data-tab');
    var tabGroup = tabBtn.closest('.tab-group');
    if (!tabGroup) return;

    // Deactivate siblings
    tabGroup.querySelectorAll('.tab-btn').forEach(function (btn) {
      btn.classList.remove('active');
      btn.setAttribute('aria-selected', 'false');
    });
    tabBtn.classList.add('active');
    tabBtn.setAttribute('aria-selected', 'true');

    // Find all sibling panels — they are next siblings of this tab-group
    var parent = tabGroup.parentElement;
    var panels = [];
    var sibling = tabGroup.nextElementSibling;
    while (sibling) {
      if (sibling.classList.contains('tab-panel')) {
        panels.push(sibling);
      } else if (sibling.classList.contains('tab-group') || sibling.classList.contains('card-section') || sibling.classList.contains('try-it-toggle') || sibling.classList.contains('response-source')) {
        break;
      }
      sibling = sibling.nextElementSibling;
    }

    panels.forEach(function (panel) {
      if (panel.id === 'panel-' + tabId) {
        panel.classList.add('active');
      } else {
        panel.classList.remove('active');
      }
    });
  });

  // ============================================================
  // Partial badge toggle
  // ============================================================
  contentInner.addEventListener('click', function (e) {
    var btn = e.target.closest('[data-partial-toggle]');
    if (!btn) return;
    var epId = btn.getAttribute('data-partial-toggle');
    var callout = document.getElementById('unresolved-' + epId);
    if (!callout) return;
    var isHidden = callout.classList.contains('hidden');
    callout.classList.toggle('hidden');
    btn.setAttribute('aria-expanded', isHidden ? 'true' : 'false');
  });

  // ============================================================
  // Copy button
  // ============================================================
  document.addEventListener('click', function (e) {
    var btn = e.target.closest('.copy-btn[data-copy-target]');
    if (!btn) return;
    var targetId = btn.getAttribute('data-copy-target');
    var codeEl = document.getElementById('code-' + targetId);
    if (!codeEl) return;
    var raw = codeEl.getAttribute('data-raw') || codeEl.textContent;
    // Decode HTML entities from data-raw
    var ta = document.createElement('textarea');
    ta.innerHTML = raw;
    copyToClipboard(ta.value, btn);
  });

  // ============================================================
  // Try It toggle
  // ============================================================
  contentInner.addEventListener('click', function (e) {
    var toggle = e.target.closest('.try-it-toggle[data-tryit]');
    if (!toggle) return;
    var epId = toggle.getAttribute('data-tryit');
    var panel = document.getElementById('tryit-' + epId);
    if (!panel) return;
    var isOpen = panel.classList.contains('open');
    panel.classList.toggle('open');
    toggle.setAttribute('aria-expanded', !isOpen);
    var chevron = toggle.querySelector('.chevron');
    if (chevron) chevron.innerHTML = isOpen ? '&#9656;' : '&#9662;';
  });

  // ============================================================
  // Send Request
  // ============================================================
  contentInner.addEventListener('click', function (e) {
    var btn = e.target.closest('.send-btn[data-send]');
    if (!btn) return;
    var epId = btn.getAttribute('data-send');
    var ep = findEndpoint(epId);
    if (!ep) return;
    var panel = document.getElementById('tryit-' + epId);
    if (!panel) return;

    // Build URL
    var url = baseUrl + ep.path;

    // Replace path params
    panel.querySelectorAll('.try-it-path-input').forEach(function (input) {
      var paramName = input.getAttribute('data-param');
      url = url.replace('{' + paramName + '}', input.value);
    });

    // Query params
    var queryParts = [];
    panel.querySelectorAll('.try-it-param-input[data-param]').forEach(function (input) {
      if (input.value) {
        queryParts.push(encodeURIComponent(input.getAttribute('data-param')) + '=' + encodeURIComponent(input.value));
      }
    });
    if (queryParts.length > 0) url += '?' + queryParts.join('&');

    // Body
    var body = null;
    var textarea = panel.querySelector('.try-it-body-textarea');
    if (textarea && textarea.value) {
      body = textarea.value;
    }

    // Headers
    var headers = {};
    if (body) {
      headers['Content-Type'] = ep.body ? ep.body.contentType : 'application/json';
    }
    panel.querySelectorAll('.try-it-param-input[data-header]').forEach(function (input) {
      if (input.value) {
        headers[input.getAttribute('data-header')] = input.value;
      }
    });

    // Auth — use the endpoint's required scheme
    var epScheme = (ep.auth && ep.auth.required && ep.auth.schemes && ep.auth.schemes.length > 0) ? ep.auth.schemes[0] : null;
    if (epScheme === 'bearer' && globalAuth.bearer) {
      headers['Authorization'] = 'Bearer ' + globalAuth.bearer;
    } else if (epScheme === 'apikey' && globalAuth.apikeyValue) {
      headers[globalAuth.apikeyHeader] = globalAuth.apikeyValue;
    } else if (epScheme === 'basic' && globalAuth.basicUser) {
      headers['Authorization'] = 'Basic ' + btoa(globalAuth.basicUser + ':' + globalAuth.basicPass);
    }

    // Spinner
    btn.disabled = true;
    var origText = btn.textContent;
    btn.innerHTML = '<span class="spinner"></span> Sending...';

    var startTime = Date.now();
    var respArea = document.getElementById('tryit-resp-' + epId);

    fetch(url, {
      method: ep.method,
      headers: headers,
      body: body
    }).then(function (resp) {
      var latency = Date.now() - startTime;
      return resp.text().then(function (text) {
        renderTryItResponse(respArea, resp.status, latency, text, resp.headers, ep, epId);
      });
    }).catch(function (err) {
      var latency = Date.now() - startTime;
      renderTryItResponse(respArea, 0, latency, 'Error: ' + err.message, null, ep, epId);
    }).finally(function () {
      btn.disabled = false;
      btn.textContent = origText;
    });
  });

  function renderTryItResponse(container, status, latency, body, headers, ep, epId) {
    container.style.display = 'block';
    var sc = status >= 200 && status < 300 ? 's2xx' : status >= 400 && status < 500 ? 's4xx' : 's5xx';

    var h = '<div class="try-it-response-header">';
    h += '<span class="try-it-response-status ' + sc + '">' + (status || 'ERR') + ' ' + statusText(status) + '</span>';
    h += '<span class="try-it-latency">' + latency + 'ms</span>';
    h += '</div>';

    // Body
    if (body) {
      var formatted = body;
      try {
        var parsed = JSON.parse(body);
        formatted = JSON.stringify(parsed, null, 2);
      } catch (e) { /* use raw */ }
      h += buildCodeBlock(formatted, epId + '-tryit-resp', 'json');

      // Check for token in 2xx response
      if (status >= 200 && status < 300) {
        try {
          var respObj = JSON.parse(body);
          var tokenFields = ['token', 'access_token', 'jwt', 'id_token', 'bearer'];
          var foundToken = null;
          tokenFields.forEach(function (field) {
            if (respObj[field] && typeof respObj[field] === 'string') {
              foundToken = respObj[field];
            }
          });
          if (foundToken) {
            h += '<button class="use-token-btn" data-token="' + esc(foundToken) + '">&#10003; Use this token globally</button>';
          }
        } catch (e) { /* not JSON */ }
      }
    }

    // Response headers toggle
    if (headers) {
      h += '<button class="try-it-resp-headers-toggle" data-resp-headers="' + epId + '">&#9656; show response headers</button>';
      h += '<div class="try-it-resp-headers" id="resp-headers-' + epId + '">';
      var headersText = '';
      headers.forEach(function (value, key) {
        headersText += key + ': ' + value + '\n';
      });
      if (headersText) {
        h += '<pre class="code-block">' + esc(headersText) + '</pre>';
      }
      h += '</div>';
    }

    container.innerHTML = h;
  }

  // Response headers toggle
  contentInner.addEventListener('click', function (e) {
    var btn = e.target.closest('.try-it-resp-headers-toggle');
    if (!btn) return;
    var id = btn.getAttribute('data-resp-headers');
    var el = document.getElementById('resp-headers-' + id);
    if (!el) return;
    var isOpen = el.classList.contains('open');
    el.classList.toggle('open');
    btn.innerHTML = (isOpen ? '&#9656;' : '&#9662;') + ' ' + (isOpen ? 'show' : 'hide') + ' response headers';
  });

  // Use token globally button
  contentInner.addEventListener('click', function (e) {
    var btn = e.target.closest('.use-token-btn');
    if (!btn) return;
    var token = btn.getAttribute('data-token');
    if (!token) return;
    globalAuth.bearer = token;
    document.getElementById('auth-bearer-token').value = token;
    updateAuthUI();
    btn.textContent = 'Token saved globally';
    btn.disabled = true;
  });

  // ============================================================
  // Authorize Modal
  // ============================================================
  var authModal = document.getElementById('auth-modal');
  var authBtn = document.getElementById('authorize-btn');
  var authClose = document.getElementById('auth-modal-close');
  var authSave = document.getElementById('auth-modal-save');
  var authClear = document.getElementById('auth-modal-clear');

  function updateModalSectionVisibility() {
    authModal.querySelectorAll('.auth-section[data-auth-scheme]').forEach(function (section) {
      var scheme = section.getAttribute('data-auth-scheme');
      section.style.display = usedSchemes.has(scheme) ? '' : 'none';
    });
  }

  function updateModalStatusIndicators() {
    var indicators = {
      bearer: document.getElementById('auth-status-bearer'),
      apikey: document.getElementById('auth-status-apikey'),
      basic: document.getElementById('auth-status-basic')
    };
    Object.keys(indicators).forEach(function (scheme) {
      var el = indicators[scheme];
      if (!el) return;
      if (hasAuthFor(scheme)) {
        el.textContent = '\u2713 Configured';
        el.className = 'auth-section-status configured';
      } else {
        el.textContent = '';
        el.className = 'auth-section-status';
      }
    });
  }

  authBtn.addEventListener('click', function () {
    authModal.classList.add('open');
    // Hide sections for schemes not used by any endpoint
    updateModalSectionVisibility();
    // Populate fields
    document.getElementById('auth-bearer-token').value = globalAuth.bearer;
    document.getElementById('auth-apikey-header').value = globalAuth.apikeyHeader;
    document.getElementById('auth-apikey-value').value = globalAuth.apikeyValue;
    document.getElementById('auth-basic-user').value = globalAuth.basicUser;
    document.getElementById('auth-basic-pass').value = globalAuth.basicPass;
    // Update status indicators
    updateModalStatusIndicators();
    // Focus first visible input
    var firstVisible = authModal.querySelector('.auth-section[data-auth-scheme]:not([style*="display: none"]) .auth-input');
    if (firstVisible) firstVisible.focus();
  });

  authClose.addEventListener('click', function () {
    authModal.classList.remove('open');
  });

  // Close on overlay click
  authModal.addEventListener('click', function (e) {
    if (e.target === authModal) authModal.classList.remove('open');
  });

  // Close on Escape
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape' && authModal.classList.contains('open')) {
      authModal.classList.remove('open');
    }
  });

  authSave.addEventListener('click', function () {
    globalAuth.bearer = document.getElementById('auth-bearer-token').value.trim();
    globalAuth.apikeyHeader = document.getElementById('auth-apikey-header').value.trim() || 'X-API-Key';
    globalAuth.apikeyValue = document.getElementById('auth-apikey-value').value.trim();
    globalAuth.basicUser = document.getElementById('auth-basic-user').value.trim();
    globalAuth.basicPass = document.getElementById('auth-basic-pass').value;
    updateModalStatusIndicators();
    authModal.classList.remove('open');
    updateAuthUI();
  });

  authClear.addEventListener('click', function () {
    document.getElementById('auth-bearer-token').value = '';
    document.getElementById('auth-apikey-header').value = 'X-API-Key';
    document.getElementById('auth-apikey-value').value = '';
    document.getElementById('auth-basic-user').value = '';
    document.getElementById('auth-basic-pass').value = '';
    globalAuth.bearer = '';
    globalAuth.apikeyValue = '';
    globalAuth.basicUser = '';
    globalAuth.basicPass = '';
    updateAuthUI();
  });

  function updateAuthUI() {
    var btnEl = document.getElementById('authorize-btn');
    var textEl = document.getElementById('authorize-btn-text');
    if (hasAuth()) {
      btnEl.classList.add('auth-active');
      textEl.textContent = 'Authorized';
    } else {
      btnEl.classList.remove('auth-active');
      textEl.textContent = 'Authorize';
    }
    // Update all Try It auth displays
    document.querySelectorAll('[id^="tryit-auth-"]').forEach(function (el) {
      var epId = el.id.replace('tryit-auth-', '');
      var ep = findEndpoint(epId);
      if (ep) el.innerHTML = renderTryItAuth(ep);
    });
    // Update all curl blocks — rebuild cards is expensive, so just update curl text
    // (Users can regenerate by toggling tabs)
  }

  // ============================================================
  // Theme toggle
  // ============================================================
  document.getElementById('theme-toggle').addEventListener('click', function () {
    var current = document.documentElement.getAttribute('data-theme');
    var next = current === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('gdl-theme', next);
  });

  // ============================================================
  // Sidebar collapse/expand
  // ============================================================
  function toggleGroup(tag) {
    var content = document.querySelector('[data-group-content="' + tag + '"]');
    var header = document.querySelector('[data-group-toggle="' + tag + '"]');
    var chevron = header ? header.querySelector('.group-chevron') : null;
    if (!content) return;

    var isCollapsed = content.classList.contains('collapsed');
    if (isCollapsed) {
      content.classList.remove('collapsed');
      if (chevron) chevron.innerHTML = '&#9662;';
      if (header) header.setAttribute('aria-expanded', 'true');
      delete collapsedState[tag];
    } else {
      content.classList.add('collapsed');
      if (chevron) chevron.innerHTML = '&#9656;';
      if (header) header.setAttribute('aria-expanded', 'false');
      collapsedState[tag] = true;
    }
    saveCollapsed(collapsedState);
  }

  sidebarGroupsEl.addEventListener('click', function (e) {
    var header = e.target.closest('[data-group-toggle]');
    if (!header) return;
    toggleGroup(header.getAttribute('data-group-toggle'));
  });

  // Keyboard support for group headers
  sidebarGroupsEl.addEventListener('keydown', function (e) {
    if (e.key === 'Enter' || e.key === ' ') {
      var header = e.target.closest('[data-group-toggle]');
      if (header) {
        e.preventDefault();
        toggleGroup(header.getAttribute('data-group-toggle'));
      }
      var row = e.target.closest('.endpoint-row');
      if (row) {
        e.preventDefault();
        row.click();
      }
    }
  });

  // ============================================================
  // Sidebar endpoint click -> scroll to card
  // ============================================================
  sidebarGroupsEl.addEventListener('click', function (e) {
    var row = e.target.closest('.endpoint-row');
    if (!row) return;
    var epId = row.getAttribute('data-ep-id');
    var card = document.getElementById('card-' + epId);
    if (!card) return;
    card.scrollIntoView({ behavior: 'smooth', block: 'start' });
    setActiveRow(epId);
  });

  // ============================================================
  // Scroll sync
  // ============================================================
  var activeRowId = null;

  function setActiveRow(epId) {
    if (activeRowId === epId) return;
    activeRowId = epId;
    sidebarGroupsEl.querySelectorAll('.endpoint-row').forEach(function (row) {
      if (row.getAttribute('data-ep-id') === epId) {
        row.classList.add('active');
        var sidebarGroups = document.getElementById('sidebar-groups');
        var rowRect = row.getBoundingClientRect();
        var sidebarRect = sidebarGroups.getBoundingClientRect();
        if (rowRect.top < sidebarRect.top || rowRect.bottom > sidebarRect.bottom) {
          row.scrollIntoView({ block: 'nearest' });
        }
      } else {
        row.classList.remove('active');
      }
    });
  }

  function onContentScroll() {
    var cards = contentInner.querySelectorAll('.endpoint-card');
    var areaTop = contentArea.getBoundingClientRect().top;
    var bestId = null;
    var bestDist = Infinity;

    cards.forEach(function (card) {
      if (card.classList.contains('hidden')) return;
      var rect = card.getBoundingClientRect();
      var dist = Math.abs(rect.top - areaTop);
      if (rect.top <= areaTop + 120 && rect.bottom > areaTop) {
        if (dist < bestDist) {
          bestDist = dist;
          bestId = card.getAttribute('data-ep-id');
        }
      }
    });

    if (!bestId) {
      cards.forEach(function (card) {
        if (bestId || card.classList.contains('hidden')) return;
        var rect = card.getBoundingClientRect();
        if (rect.bottom > areaTop && rect.top < areaTop + contentArea.clientHeight) {
          bestId = card.getAttribute('data-ep-id');
        }
      });
    }

    if (bestId) setActiveRow(bestId);
  }

  contentArea.addEventListener('scroll', onContentScroll);
  requestAnimationFrame(onContentScroll);

  // ============================================================
  // Search
  // ============================================================
  var searchInput = document.getElementById('search-input');
  var searchShortcut = document.getElementById('search-shortcut');

  // Platform-aware search shortcut display.
  var isMac = /Mac|iPod|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
  searchShortcut.textContent = isMac ? '\u2318K' : 'Ctrl+K';

  document.addEventListener('keydown', function (e) {
    // Cmd+K (Mac) / Ctrl+K (others) — focus search.
    if (e.key === 'k' && (isMac ? e.metaKey : e.ctrlKey)) {
      e.preventDefault();
      searchInput.focus();
      return;
    }
    if (e.key === '/' && document.activeElement !== searchInput) {
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;
      e.preventDefault();
      searchInput.focus();
    }
    if (e.key === 'Escape' && document.activeElement === searchInput) {
      searchInput.value = '';
      applySearch('');
      searchInput.blur();
    }
  });

  searchInput.addEventListener('focus', function () { searchShortcut.style.display = 'none'; });
  searchInput.addEventListener('blur', function () { if (!searchInput.value) searchShortcut.style.display = ''; });
  searchInput.addEventListener('input', function () { applySearch(searchInput.value); });

  function applySearch(query) {
    var q = query.toLowerCase().trim();
    var groups = sidebarGroupsEl.querySelectorAll('.sidebar-group');

    groups.forEach(function (groupEl) {
      var rows = groupEl.querySelectorAll('.endpoint-row');
      var visibleCount = 0;

      rows.forEach(function (row) {
        var epId = row.getAttribute('data-ep-id');
        var ep = findEndpoint(epId);
        if (!ep) return;

        var match = !q ||
          ep.method.toLowerCase().indexOf(q) !== -1 ||
          ep.path.toLowerCase().indexOf(q) !== -1 ||
          (ep.summary && ep.summary.toLowerCase().indexOf(q) !== -1) ||
          (ep.tag && ep.tag.toLowerCase().indexOf(q) !== -1);

        if (match) { row.classList.remove('hidden'); visibleCount++; }
        else { row.classList.add('hidden'); }
      });

      if (q && visibleCount === 0) {
        groupEl.classList.add('hidden');
      } else {
        groupEl.classList.remove('hidden');
        if (q && visibleCount > 0) {
          var content = groupEl.querySelector('.group-content');
          if (content) content.classList.remove('collapsed');
          var chevron = groupEl.querySelector('.group-chevron');
          if (chevron) chevron.innerHTML = '&#9662;';
        }
      }
    });

    // Filter cards
    contentInner.querySelectorAll('.endpoint-card').forEach(function (card) {
      var epId = card.getAttribute('data-ep-id');
      var ep = findEndpoint(epId);
      if (!ep) return;
      var match = !q ||
        ep.method.toLowerCase().indexOf(q) !== -1 ||
        ep.path.toLowerCase().indexOf(q) !== -1 ||
        (ep.summary && ep.summary.toLowerCase().indexOf(q) !== -1) ||
        (ep.tag && ep.tag.toLowerCase().indexOf(q) !== -1);
      if (match) card.classList.remove('hidden');
      else card.classList.add('hidden');
    });

    if (!q) {
      groups.forEach(function (groupEl) {
        var tag = groupEl.getAttribute('data-group');
        var content = groupEl.querySelector('.group-content');
        var chevron = groupEl.querySelector('.group-chevron');
        var header = groupEl.querySelector('.group-header');
        if (collapsedState[tag]) {
          if (content) content.classList.add('collapsed');
          if (chevron) chevron.innerHTML = '&#9656;';
          if (header) header.setAttribute('aria-expanded', 'false');
        }
      });
    }
  }

  // ============================================================
  // Endpoint lookup
  // ============================================================
  function findEndpoint(epId) {
    var parts = epId.split('-');
    var idx = parseInt(parts[parts.length - 1], 10);
    var tag = parts.slice(0, parts.length - 1).join('-');
    var eps = grouped.groups[tag];
    return eps ? eps[idx] : null;
  }

  // ============================================================
  // Unresolved filter (sidebar footer)
  // ============================================================
  sidebarFooterEl.addEventListener('click', function (e) {
    var btn = e.target.closest('#unresolved-filter');
    if (!btn) return;
    if (searchInput.value === ':unresolved') {
      searchInput.value = '';
      applySearch('');
    } else {
      searchInput.value = ':unresolved';
      applyUnresolvedFilter(true);
    }
  });

  sidebarFooterEl.addEventListener('keydown', function (e) {
    if ((e.key === 'Enter' || e.key === ' ') && e.target.id === 'unresolved-filter') {
      e.preventDefault();
      e.target.click();
    }
  });

  function applyUnresolvedFilter(active) {
    if (!active) { applySearch(''); return; }
    var groups = sidebarGroupsEl.querySelectorAll('.sidebar-group');
    groups.forEach(function (groupEl) {
      var rows = groupEl.querySelectorAll('.endpoint-row');
      var visibleCount = 0;
      rows.forEach(function (row) {
        var ep = findEndpoint(row.getAttribute('data-ep-id'));
        if (!ep) return;
        if (ep.unresolved && ep.unresolved.length > 0) { row.classList.remove('hidden'); visibleCount++; }
        else { row.classList.add('hidden'); }
      });
      if (visibleCount === 0) groupEl.classList.add('hidden');
      else {
        groupEl.classList.remove('hidden');
        var c = groupEl.querySelector('.group-content');
        if (c) c.classList.remove('collapsed');
      }
    });
    contentInner.querySelectorAll('.endpoint-card').forEach(function (card) {
      var ep = findEndpoint(card.getAttribute('data-ep-id'));
      if (!ep) return;
      if (ep.unresolved && ep.unresolved.length > 0) card.classList.remove('hidden');
      else card.classList.add('hidden');
    });
  }

  // ============================================================
  // Sidebar toggle (mobile)
  // ============================================================
  var sidebarEl = document.getElementById('sidebar');
  var sidebarOverlay = document.getElementById('sidebar-overlay');
  var sidebarToggleBtn = document.getElementById('sidebar-toggle');

  function openSidebar() {
    sidebarEl.classList.add('open');
    sidebarOverlay.classList.add('open');
    sidebarToggleBtn.setAttribute('aria-expanded', 'true');
  }

  function closeSidebar() {
    sidebarEl.classList.remove('open');
    sidebarOverlay.classList.remove('open');
    sidebarToggleBtn.setAttribute('aria-expanded', 'false');
  }

  sidebarToggleBtn.addEventListener('click', function () {
    if (sidebarEl.classList.contains('open')) closeSidebar();
    else openSidebar();
  });

  sidebarOverlay.addEventListener('click', closeSidebar);

  // Close sidebar on endpoint click on mobile
  sidebarGroupsEl.addEventListener('click', function (e) {
    if (e.target.closest('.endpoint-row') && window.innerWidth <= 768) {
      closeSidebar();
    }
  });

  // ============================================================
  // SSE live reload — connect when served via godoclive watch
  // ============================================================
  if (window.location.protocol !== 'file:') {
    try {
      var evtSource = new EventSource('/events');
      evtSource.addEventListener('reload', function () {
        window.location.reload();
      });
    } catch (e) {
      // SSE not available — static file mode, ignore.
    }
  }

})();
