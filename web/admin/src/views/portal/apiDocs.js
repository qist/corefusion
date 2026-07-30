// API reference content shown in Settings → API tab of the portal builder.
// Pure data builder so the view stays thin.

export function buildApiGroups(t) {
  return [
    {
      title: t('portal.api.group.portalMgmt'),
      apis: [
        {
          method: 'GET',
          path: '/api/v1/portal/active',
          desc: t('portal.api.getActive'),
          auth: 'Authorization: Bearer <admin_token>',
          requestBody: null,
          response: `{
  "code": 0,
  "data": {
    "id": 1, "name": "Default", "is_active": true,
    "site_name": "CoreFusion",
    "logo_url": "",
    "primary_color": "#409eff",
    "background_color": "#f5f7fa",
    "card_background_color": "#ffffff",
    "text_color": "#303133",
    "accent_color": "#67c23a",
    "font_family": "-apple-system, ...",
    "portal_editor_version": 1,
    "registration_enabled": true,
    "navigation": [
      { "id": "nav-overview", "label": "${t('portal.api.ex.navOverview')}", "icon": "📊", "target": "overview", "href": "", "enabled": true, "open_external": false }
    ],
    "modules": [
      { "id": "mod-hero", "type": "hero", "title": "${t('portal.api.ex.heroTitle')}", "visible": true,
        "data": { "eyebrow": "CORE FUSION", "heading": "${t('portal.api.ex.heroHeading')}", "description": "..." },
        "style": { "variant": "primary", "align": "left" } }
    ]
  }
}`,
        },
        {
          method: 'PUT',
          path: '/api/v1/portal/active',
          desc: t('portal.api.putActive'),
          auth: 'Authorization: Bearer <admin_token>',
          requestBody: `{
  "name": "MyPortal",
  "description": "${t('portal.api.ex.siteDesc')}",
  "navigation": [
    { "id": "nav-home", "label": "${t('portal.api.ex.navHome')}", "icon": "🏠", "target": "overview", "href": "", "enabled": true, "open_external": false }
  ],
  "modules": [
    { "id": "mod-hero", "type": "hero", "title": "${t('portal.api.ex.heroTitle')}", "visible": true,
      "data": { "eyebrow": "WELCOME", "heading": "${t('portal.api.ex.heroTitle')}", "description": "...", "button_text": "${t('portal.api.ex.buttonView')}", "button_target": "subscription" },
      "style": { "variant": "primary", "align": "left" } }
  ],
  "theme": {
    "site_name": "MyPortal",
    "logo_url": "",
    "primary_color": "#ff6600",
    "background_color": "#f5f5f5",
    "card_background_color": "#ffffff",
    "text_color": "#333333",
    "accent_color": "#00cc00",
    "font_family": "sans-serif"
  },
  "registration_enabled": true,
  "portal_port": ""
}`,
          response: `{
  "code": 0,
  "data": { "id": 1, "name": "MyPortal", "site_name": "MyPortal", "primary_color": "#ff6600", "navigation": [ ... ], "modules": [ ... ] }
}`,
        },
        {
          method: 'POST',
          path: '/api/v1/portal/reset',
          desc: t('portal.api.reset'),
          auth: 'Authorization: Bearer <admin_token>',
          requestBody: null,
          response: `{
  "code": 0,
  "data": { "name": "Default", "site_name": "CoreFusion", "navigation": [ ... ], "modules": [ ... ] }
}`,
        },
        {
          method: 'GET',
          path: '/api/v1/portal/cache',
          desc: t('portal.api.getCache'),
          auth: 'Authorization: Bearer <admin_token>',
          requestBody: null,
          response: `{
  "code": 0,
  "data": { "cached": true, "cache_key": "a1b2c3d4e5f6a7b8", "built_at": "2025-01-01T12:00:00Z", "hit_count": 42, "miss_count": 1, "dev_mode": false }
}`,
        },
        {
          method: 'DELETE',
          path: '/api/v1/portal/cache',
          desc: t('portal.api.deleteCache'),
          auth: 'Authorization: Bearer <admin_token>',
          requestBody: null,
          response: `{ "code": 0, "data": { "message": "cache cleared" } }`,
        },
        {
          method: 'POST',
          path: '/api/v1/portal/cache/rebuild',
          desc: t('portal.api.rebuildCache'),
          auth: 'Authorization: Bearer <admin_token>',
          requestBody: null,
          response: `{ "code": 0, "data": { "message": "cache rebuilt", "stats": { "cached": true, "hit_count": 42, "miss_count": 2, "dev_mode": false } } }`,
        },
        {
          method: 'PUT',
          path: '/api/v1/portal/cache/dev-mode',
          desc: t('portal.api.devMode'),
          auth: 'Authorization: Bearer <admin_token>',
          requestBody: `{ "dev_mode": true }`,
          response: `{ "code": 0, "data": { "message": "dev mode set to dev", "dev_mode": true } }`,
        },
        {
          method: 'GET',
          path: '/api/v1/portal/templates',
          desc: t('portal.api.getTemplates'),
          auth: 'Authorization: Bearer <admin_token>',
          requestBody: null,
          response: `{
  "code": 0,
  "data": [
    { "id": 1, "name": "Default", "is_active": true, "is_built_in": true }
  ]
}`,
        },
        {
          method: 'POST',
          path: '/api/v1/portal/templates',
          desc: t('portal.api.postTemplates'),
          auth: 'Authorization: Bearer <admin_token>',
          requestBody: `{
  "name": "${t('portal.api.ex.templateName')}",
  "description": "${t('portal.api.ex.customPortalDesc')}",
  "html_content": "<div>...</div>",
  "css_content": "body { ... }",
  "js_content": "console.log('hi')",
  "config": "{ ... }"
}`,
          response: `{
  "code": 0,
  "data": { "id": 2, "name": "${t('portal.api.ex.templateName')}", "is_active": false }
}`,
        },
        {
          method: 'POST',
          path: '/api/v1/portal/templates/:id/activate',
          desc: t('portal.api.activateTemplate'),
          auth: 'Authorization: Bearer <admin_token>',
          requestBody: null,
          response: `{ "code": 0, "data": { "message": "activated", "id": 2 } }`,
        },
        {
          method: 'DELETE',
          path: '/api/v1/portal/templates/:id',
          desc: t('portal.api.deleteTemplate'),
          auth: 'Authorization: Bearer <admin_token>',
          requestBody: null,
          response: `{ "code": 0, "data": { "message": "deleted" } }`,
        },
      ],
    },
    {
      title: t('portal.api.group.public'),
      apis: [
        {
          method: 'GET',
          path: '/api/v1/public/portal-config',
          desc: t('portal.api.publicPortalConfig'),
          auth: t('portal.api.noAuth'),
          requestBody: null,
          response: `{
  "code": 0,
  "data": {
    "site_name": "CoreFusion",
    "primary_color": "#667eea",
    "registration_enabled": true,
    "navigation": [ ... ],
    "modules": [ ... ],
    "lang": { "default": "zh-CN", "locales": { ... } },
    "regions": { "auth": { "html": "...", "css": "...", "js": "..." } }
  }
}`,
        },
        {
          method: 'GET',
          path: '/api/v1/public/settings',
          desc: t('portal.api.publicSettings'),
          auth: t('portal.api.noAuth'),
          requestBody: null,
          response: `{
  "code": 0,
  "data": { "registration_enabled": true }
}`,
        },
        {
          method: 'GET',
          path: '/api/v1/public/subscription-endpoints',
          desc: t('portal.api.publicSubEndpoints'),
          auth: t('portal.api.noAuth'),
          requestBody: null,
          response: `{
  "code": 0,
  "data": [
    { "id": 1, "name": "${t('portal.api.ex.endpointName')}", "host": "example.com", "port": 8081 }
  ]
}`,
        },
      ],
    },
    {
      title: t('portal.api.group.client'),
      apis: [
        {
          method: 'POST',
          path: '/api/v1/client-auth/register',
          desc: t('portal.api.register'),
          auth: t('portal.api.noAuth'),
          requestBody: `{
  "username": "user1",
  "password": "MyPass123!",
  "email": "user@example.com"
}`,
          response: `{
  "code": 0,
  "data": { "message": "${t('portal.api.ex.msgRegister')}" }
}`,
        },
        {
          method: 'POST',
          path: '/api/v1/client-auth/login',
          desc: t('portal.api.login'),
          auth: t('portal.api.noAuth'),
          requestBody: `{
  "username": "user1",
  "password": "MyPass123!"
}`,
          response: `{
  "code": 0,
  "data": {
    "access_token": "eyJhbGci...",
    "refresh_token": "eyJhbGci...",
    "expires_in": 86400
  }
}`,
        },
        {
          method: 'GET',
          path: '/api/v1/client-auth/me',
          desc: t('portal.api.me'),
          auth: 'Authorization: Bearer <client_token>',
          requestBody: null,
          response: `{
  "code": 0,
  "data": {
    "id": 1, "username": "user1",
    "subscription_url": "https://example.com/sub/abc123",
    "traffic_used": 1073741824,
    "traffic_total": 536870912000
  }
}`,
        },
        {
          method: 'POST',
          path: '/api/v1/client-auth/rotate-token',
          desc: t('portal.api.rotateToken'),
          auth: 'Authorization: Bearer <client_token>',
          requestBody: null,
          response: `{
  "code": 0,
  "data": { "subscription_url": "https://example.com/sub/xyz789" }
}`,
        },
        {
          method: 'PUT',
          path: '/api/v1/client-auth/settings',
          desc: t('portal.api.updateSettings'),
          auth: 'Authorization: Bearer <client_token>',
          requestBody: `{
  "alert_on_traffic_limit": true,
  "traffic_alert_threshold": 90
}`,
          response: `{ "code": 0, "data": { "message": "${t('portal.api.ex.msgSettings')}" } }`,
        },
        {
          method: 'POST',
          path: '/api/v1/client-auth/change-password',
          desc: t('portal.api.changePassword'),
          auth: 'Authorization: Bearer <client_token>',
          requestBody: `{
  "old_password": "OldPass123!",
  "new_password": "NewPass456!"
}`,
          response: `{ "code": 0, "data": { "message": "${t('portal.api.ex.msgPassword')}" } }`,
        },
      ],
    },
  ]
}
