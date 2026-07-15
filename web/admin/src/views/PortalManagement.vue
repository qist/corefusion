<template>
  <div class="portal-management">
    <!-- Top Bar -->
    <div class="editor-topbar">
      <div class="topbar-left">
        <span class="portal-name">{{ form.name || '门户站点' }}</span>
        <el-tag v-if="hasUnsavedChanges" type="warning" size="small">未保存</el-tag>
        <el-tag v-else type="success" size="small">已保存</el-tag>
      </div>
      <div class="topbar-right">
        <el-button-group>
          <el-button :type="editorMode === 'builder' ? 'primary' : 'default'" size="small" @click="editorMode = 'builder'">搭建器</el-button>
          <el-button :type="editorMode === 'settings' ? 'primary' : 'default'" size="small" @click="editorMode = 'settings'">设置</el-button>
          <el-button :type="editorMode === 'code' ? 'primary' : 'default'" size="small" @click="editorMode = 'code'">模板编辑</el-button>
        </el-button-group>
        <el-button size="small" :icon="View" @click="handlePreview">预览</el-button>
        <el-button type="primary" size="small" :icon="Check" :loading="saving" @click="handleSave">保存</el-button>
      </div>
    </div>

    <div v-loading="loading" class="editor-body">
      <!-- ===== Builder Mode (Three-Column) ===== -->
      <div v-if="editorMode === 'builder'" class="builder-layout">
        <!-- Left Column: Page Structure -->
        <div class="col-structure">
          <div class="structure-section">
            <div class="section-header">
              <span class="section-title">导航菜单</span>
              <el-button text size="small" :icon="Plus" @click="openNavDialog()" />
            </div>
            <div class="nav-list">
              <div
                v-for="(nav, idx) in navigation"
                :key="nav.id"
                class="nav-item-row"
                :class="{ selected: selectedNavId === nav.id, disabled: !nav.enabled }"
                @click="selectNav(nav.id)"
              >
                <div class="nav-item-info">
                  <span class="nav-icon-emoji">{{ nav.icon || '🔗' }}</span>
                  <span class="nav-label-text">{{ nav.label }}</span>
                  <el-tag v-if="nav.target && !nav.href" size="small" type="info">{{ nav.target }}</el-tag>
                </div>
                <div class="nav-item-actions" @click.stop>
                  <el-button text size="small" :icon="ArrowUp" @click="moveItem(navigation, idx, -1)" :disabled="idx === 0" />
                  <el-button text size="small" :icon="ArrowDown" @click="moveItem(navigation, idx, 1)" :disabled="idx === navigation.length - 1" />
                  <el-switch v-model="nav.enabled" size="small" />
                  <el-button text size="small" type="danger" :icon="Delete" @click="removeNav(idx)" />
                </div>
              </div>
              <div v-if="navigation.length === 0" class="empty-hint" @click="openNavDialog()">
                + 添加导航项
              </div>
            </div>
          </div>

          <div class="structure-section">
            <div class="section-header">
              <span class="section-title">内容模块</span>
              <el-dropdown @command="addModule" trigger="click">
                <el-button text size="small" :icon="Plus">添加</el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="hero">Hero 欢迎区</el-dropdown-item>
                    <el-dropdown-item command="markdown">Markdown 文档</el-dropdown-item>
                    <el-dropdown-item command="notice">公告提示</el-dropdown-item>
                    <el-dropdown-item command="feature_cards">功能卡片</el-dropdown-item>
                    <el-dropdown-item command="account_stats">账户数据</el-dropdown-item>
                    <el-dropdown-item command="subscription">订阅入口</el-dropdown-item>
                    <el-dropdown-item command="rich_text" divided>富文本 (HTML)</el-dropdown-item>
                    <el-dropdown-item command="custom_html">自定义 HTML</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
            <div class="module-list">
              <div
                v-for="(mod, idx) in modules"
                :key="mod.id"
                class="module-item-row"
                :class="{ selected: selectedModuleId === mod.id, disabled: !mod.visible }"
                @click="selectModule(mod.id)"
              >
                <div class="module-item-info">
                  <span class="module-type-badge" :class="'badge-' + mod.type">{{ moduleTypeLabel(mod.type) }}</span>
                  <span class="module-title-text">{{ mod.title || '(无标题)' }}</span>
                </div>
                <div class="module-item-actions" @click.stop>
                  <el-button text size="small" :icon="ArrowUp" @click="moveItem(modules, idx, -1)" :disabled="idx === 0" />
                  <el-button text size="small" :icon="ArrowDown" @click="moveItem(modules, idx, 1)" :disabled="idx === modules.length - 1" />
                  <el-switch v-model="mod.visible" size="small" />
                  <el-button text size="small" :icon="CopyDocument" @click="duplicateModule(idx)" />
                  <el-button text size="small" type="danger" :icon="Delete" @click="removeModule(idx)" />
                </div>
              </div>
              <div v-if="modules.length === 0" class="empty-hint" @click="addModule('hero')">
                + 添加第一个模块
              </div>
            </div>
          </div>
        </div>

        <!-- Center Column: Canvas Preview -->
        <div class="col-canvas">
          <div class="canvas-toolbar">
            <el-radio-group v-model="previewDevice" size="small">
              <el-radio-button label="desktop">桌面</el-radio-button>
              <el-radio-button label="tablet">平板</el-radio-button>
              <el-radio-button label="mobile">手机</el-radio-button>
            </el-radio-group>
          </div>
          <div class="canvas-scroll" :class="'device-' + previewDevice">
            <div class="portal-preview" :style="previewStyles">
              <!-- Site Header Preview -->
              <div class="preview-header">
                <div class="preview-header-left">
                  <span v-if="config.logo_url" class="preview-logo">🌐</span>
                  <span class="preview-site-name">{{ config.site_name || 'CoreFusion' }}</span>
                </div>
                <div class="preview-header-right">
                  <span v-for="nav in navigation.filter(n => n.enabled)" :key="nav.id" class="preview-nav-item">
                    {{ nav.label }}
                  </span>
                </div>
              </div>

              <!-- Module Previews -->
              <div class="preview-modules">
                <template v-for="mod in modules.filter(m => m.visible)" :key="mod.id">
                  <!-- Hero Module Preview -->
                  <div v-if="mod.type === 'hero'" class="preview-module preview-hero" :style="moduleStyle(mod)">
                    <div class="hero-eyebrow">{{ mod.data?.eyebrow || '' }}</div>
                    <h2 class="hero-heading">{{ mod.data?.heading || mod.title }}</h2>
                    <p class="hero-desc">{{ mod.data?.description || '' }}</p>
                    <button v-if="mod.data?.button_text" class="hero-btn">{{ mod.data.button_text }}</button>
                  </div>

                  <!-- Markdown Module Preview -->
                  <div v-else-if="mod.type === 'markdown'" class="preview-module preview-markdown" :style="moduleStyle(mod)">
                    <div v-if="mod.title" class="module-section-title">{{ mod.title }}</div>
                    <div class="md-preview-content" v-html="renderMd(mod.data?.markdown || '')"></div>
                  </div>

                  <!-- Rich Text Module Preview -->
                  <div v-else-if="mod.type === 'rich_text'" class="preview-module preview-rich-text" :style="moduleStyle(mod)">
                    <div v-html="sanitizeHtml(mod.data?.html || '')"></div>
                  </div>

                  <!-- Notice Module Preview -->
                  <div v-else-if="mod.type === 'notice'" class="preview-module preview-notice" :class="'notice-' + (mod.style?.variant || 'info')">
                    <div class="notice-title">{{ mod.data?.title || mod.title }}</div>
                    <div v-if="mod.data?.markdown" class="notice-body md-preview-content" v-html="renderMd(mod.data.markdown)"></div>
                    <div v-else-if="mod.data?.html" class="notice-body" v-html="sanitizeHtml(mod.data.html)"></div>
                  </div>

                  <!-- Feature Cards Module Preview -->
                  <div v-else-if="mod.type === 'feature_cards'" class="preview-module preview-feature-cards">
                    <h3 v-if="mod.title" class="module-section-title">{{ mod.title }}</h3>
                    <div class="feature-cards-grid">
                      <div v-for="(card, ci) in (mod.data?.cards || []).slice(0, 6)" :key="ci" class="feature-card">
                        <span class="feature-card-icon">{{ card.icon || '⭐' }}</span>
                        <h4>{{ card.title }}</h4>
                        <p>{{ card.description }}</p>
                      </div>
                    </div>
                  </div>

                  <!-- Account Stats Module Preview -->
                  <div v-else-if="mod.type === 'account_stats'" class="preview-module preview-account-stats">
                    <h3 v-if="mod.title" class="module-section-title">{{ mod.title }}</h3>
                    <div class="stats-grid">
                      <div class="stat-box"><span class="stat-num">{{ '--' }}</span><span class="stat-label">上传</span></div>
                      <div class="stat-box"><span class="stat-num">{{ '--' }}</span><span class="stat-label">下载</span></div>
                      <div class="stat-box"><span class="stat-num">{{ '--' }}</span><span class="stat-label">总量</span></div>
                    </div>
                  </div>

                  <!-- Subscription Module Preview -->
                  <div v-else-if="mod.type === 'subscription'" class="preview-module preview-subscription">
                    <h3 class="module-section-title">{{ mod.data?.title || mod.title || '订阅信息' }}</h3>
                    <p class="sub-desc">{{ mod.data?.description || '查看您的订阅链接和状态' }}</p>
                    <button v-if="mod.data?.button_text" class="sub-btn">{{ mod.data.button_text }}</button>
                  </div>

                  <!-- Custom HTML Module Preview -->
                  <div v-else-if="mod.type === 'custom_html'" class="preview-module preview-custom-html">
                    <div class="custom-html-badge">HTML</div>
                    <div v-html="sanitizeHtml(mod.data?.html || '')"></div>
                  </div>
                </template>

                <div v-if="modules.filter(m => m.visible).length === 0" class="preview-empty">
                  <span style="font-size: 40px">📋</span>
                  <p>从左侧添加内容模块来构建门户页面</p>
                </div>
              </div>

              <!-- Bottom Nav Preview -->
              <div class="preview-bottom-nav">
                <div v-for="nav in navigation.filter(n => n.enabled)" :key="nav.id" class="preview-bottom-nav-item">
                  <span class="nav-icon">{{ nav.icon || '🔗' }}</span>
                  <span class="nav-label">{{ nav.label }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Right Column: Properties Panel -->
        <div class="col-properties">
          <!-- Navigation Properties -->
          <div v-if="selectedNavId && selectedNav" class="props-panel">
            <h3 class="props-title">编辑导航项</h3>
            <el-form label-position="top" size="small">
              <el-form-item label="显示名称"><el-input v-model="selectedNav.label" placeholder="如：首页" /></el-form-item>
              <el-form-item label="图标 (emoji 或文字)"><el-input v-model="selectedNav.icon" placeholder="如：📊" /></el-form-item>
              <el-form-item label="目标页面">
                <el-select v-model="selectedNav.target" placeholder="选择内置页面" style="width: 100%">
                  <el-option label="概览" value="overview" /><el-option label="订阅" value="subscription" /><el-option label="安全" value="security" /><el-option label="日志" value="logs" />
                </el-select>
              </el-form-item>
              <el-form-item label="外部链接 (可选)"><el-input v-model="selectedNav.href" placeholder="https://example.com" /></el-form-item>
              <el-form-item label="新窗口打开"><el-switch v-model="selectedNav.open_external" /></el-form-item>
              <el-form-item label="显示"><el-switch v-model="selectedNav.enabled" /></el-form-item>
            </el-form>
          </div>

          <!-- Module Properties -->
          <div v-else-if="selectedModuleId && selectedModule" class="props-panel">
            <h3 class="props-title">编辑：{{ moduleTypeLabel(selectedModule.type) }}</h3>
            <el-form label-position="top" size="small">
              <el-form-item label="模块标题"><el-input v-model="selectedModule.title" placeholder="模块标题" /></el-form-item>
              <el-form-item label="显示模块"><el-switch v-model="selectedModule.visible" /></el-form-item>

              <!-- Hero Properties -->
              <template v-if="selectedModule.type === 'hero'">
                <el-form-item label="眉题"><el-input v-model="selectedModule.data.eyebrow" placeholder="如：CORE FUSION" /></el-form-item>
                <el-form-item label="标题"><el-input v-model="selectedModule.data.heading" placeholder="如：欢迎回来" /></el-form-item>
                <el-form-item label="描述"><el-input v-model="selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
                <el-form-item label="按钮文案"><el-input v-model="selectedModule.data.button_text" placeholder="如：查看订阅" /></el-form-item>
                <el-form-item label="按钮跳转">
                  <el-select v-model="selectedModule.data.button_target" style="width: 100%">
                    <el-option label="概览" value="overview" /><el-option label="订阅" value="subscription" /><el-option label="安全" value="security" /><el-option label="日志" value="logs" />
                  </el-select>
                </el-form-item>
                <el-form-item label="对齐方式">
                  <el-radio-group v-model="selectedModule.style.align">
                    <el-radio label="left">左对齐</el-radio><el-radio label="center">居中</el-radio>
                  </el-radio-group>
                </el-form-item>
                <el-form-item label="样式变体">
                  <el-select v-model="selectedModule.style.variant" style="width: 100%">
                    <el-option label="主色" value="primary" /><el-option label="渐变" value="gradient" /><el-option label="简洁" value="minimal" />
                  </el-select>
                </el-form-item>
              </template>

              <!-- Markdown Properties -->
              <template v-if="selectedModule.type === 'markdown'">
                <div class="md-editor-wrap">
                  <div class="md-editor-toolbar">
                    <button @click="insertMd('heading')" title="标题">H</button>
                    <button @click="insertMd('bold')" title="粗体"><b>B</b></button>
                    <button @click="insertMd('italic')" title="斜体"><i>I</i></button>
                    <button @click="insertMd('strikethrough')" title="删除线"><s>S</s></button>
                    <span class="md-divider"></span>
                    <button @click="insertMd('ul')" title="无序列表">•</button>
                    <button @click="insertMd('ol')" title="有序列表">1.</button>
                    <button @click="insertMd('checklist')" title="清单">☑</button>
                    <span class="md-divider"></span>
                    <button @click="insertMd('link')" title="链接">🔗</button>
                    <button @click="insertMd('image')" title="图片">🖼</button>
                    <button @click="insertMd('code')" title="代码">&lt;/&gt;</button>
                    <button @click="insertMd('codeblock')" title="代码块">{ }</button>
                    <button @click="insertMd('quote')" title="引用">❝</button>
                    <button @click="insertMd('table')" title="表格">⊞</button>
                    <button @click="insertMd('hr')" title="分割线">—</button>
                  </div>
                  <div class="md-split">
                    <textarea
                      ref="mdEditRef"
                      v-model="selectedModule.data.markdown"
                      class="md-textarea"
                      placeholder="在此编写 Markdown 内容..."
                      @input="onMdInput"
                    ></textarea>
                    <div class="md-preview-pane md-preview-content" v-html="renderMd(selectedModule.data.markdown || '')"></div>
                  </div>
                </div>
                <el-form-item label="对齐方式" style="margin-top: 8px">
                  <el-radio-group v-model="selectedModule.style.align">
                    <el-radio label="left">左</el-radio><el-radio label="center">中</el-radio>
                  </el-radio-group>
                </el-form-item>
              </template>

              <!-- Notice Properties -->
              <template v-if="selectedModule.type === 'notice'">
                <el-form-item label="公告标题"><el-input v-model="selectedModule.data.title" placeholder="公告标题" /></el-form-item>
                <div class="md-editor-wrap md-editor-compact">
                  <div class="md-editor-toolbar">
                    <button @click="insertNoticeMd('bold')" title="粗体"><b>B</b></button>
                    <button @click="insertNoticeMd('italic')" title="斜体"><i>I</i></button>
                    <button @click="insertNoticeMd('link')" title="链接">🔗</button>
                    <button @click="insertNoticeMd('code')" title="代码">&lt;/&gt;</button>
                  </div>
                  <div class="md-split md-split-compact">
                    <textarea
                      ref="noticeMdRef"
                      v-model="selectedModule.data.markdown"
                      class="md-textarea"
                      placeholder="公告正文 (支持 Markdown)..."
                      @input="onNoticeMdInput"
                    ></textarea>
                    <div class="md-preview-pane md-preview-content" v-html="renderMd(selectedModule.data.markdown || '')"></div>
                  </div>
                </div>
                <el-form-item label="状态色" style="margin-top: 8px">
                  <el-select v-model="selectedModule.style.variant" style="width: 100%">
                    <el-option label="信息" value="info" /><el-option label="成功" value="success" /><el-option label="警告" value="warning" /><el-option label="错误" value="error" />
                  </el-select>
                </el-form-item>
              </template>

              <!-- Feature Cards Properties -->
              <template v-if="selectedModule.type === 'feature_cards'">
                <div v-for="(card, ci) in selectedModule.data.cards" :key="ci" class="card-edit-row">
                  <el-input v-model="card.icon" placeholder="图标" style="width: 60px" />
                  <el-input v-model="card.title" placeholder="标题" />
                  <el-input v-model="card.description" placeholder="描述" />
                  <el-button text type="danger" :icon="Delete" @click="selectedModule.data.cards.splice(ci, 1)" />
                </div>
                <el-button text :icon="Plus" @click="selectedModule.data.cards.push({ icon: '⭐', title: '', description: '' })">添加卡片</el-button>
              </template>

              <!-- Account Stats Properties -->
              <template v-if="selectedModule.type === 'account_stats'">
                <el-form-item label="布局">
                  <el-radio-group v-model="selectedModule.style.layout">
                    <el-radio label="grid">网格</el-radio><el-radio label="row">横排</el-radio>
                  </el-radio-group>
                </el-form-item>
              </template>

              <!-- Subscription Properties -->
              <template v-if="selectedModule.type === 'subscription'">
                <el-form-item label="标题"><el-input v-model="selectedModule.data.title" placeholder="订阅信息" /></el-form-item>
                <el-form-item label="说明文字"><el-input v-model="selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
                <el-form-item label="按钮文案"><el-input v-model="selectedModule.data.button_text" placeholder="复制订阅链接" /></el-form-item>
              </template>

              <!-- Rich Text Properties -->
              <template v-if="selectedModule.type === 'rich_text'">
                <el-form-item label="富文本内容">
                  <div class="rich-text-editor">
                    <div class="rte-toolbar">
                      <button @click="execCmd('formatBlock', '<h2>')" title="标题1">H2</button>
                      <button @click="execCmd('formatBlock', '<h3>')" title="标题2">H3</button>
                      <button @click="execCmd('formatBlock', '<p>')" title="正文">P</button>
                      <span class="rte-divider"></span>
                      <button @click="execCmd('bold')" title="粗体"><b>B</b></button>
                      <button @click="execCmd('italic')" title="斜体"><i>I</i></button>
                      <button @click="execCmd('underline')" title="下划线"><u>U</u></button>
                      <button @click="execCmd('strikeThrough')" title="删除线"><s>S</s></button>
                      <span class="rte-divider"></span>
                      <button @click="execCmd('insertUnorderedList')" title="无序列表">•</button>
                      <button @click="execCmd('insertOrderedList')" title="有序列表">1.</button>
                      <button @click="execCmd('formatBlock', '<blockquote>')" title="引用">❝</button>
                      <span class="rte-divider"></span>
                      <button @click="insertLink" title="超链接">🔗</button>
                      <button @click="execCmd('removeFormat')" title="清除格式">✕</button>
                    </div>
                    <div ref="richTextEditRef" class="rte-content" contenteditable="true" @input="onRichTextInput" v-html="selectedModule.data.html || ''"></div>
                  </div>
                </el-form-item>
              </template>

              <!-- Custom HTML Properties -->
              <template v-if="selectedModule.type === 'custom_html'">
                <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 8px">
                  <span style="font-size: 12px">自定义 HTML 会被直接渲染，请确保内容安全</span>
                </el-alert>
                <el-form-item label="HTML 内容">
                  <el-input v-model="selectedModule.data.html" type="textarea" :rows="12" placeholder="<div>自定义内容</div>" />
                </el-form-item>
              </template>
            </el-form>
          </div>

          <!-- No Selection -->
          <div v-else class="props-empty">
            <span class="props-empty-icon">⚙️</span>
            <p>选择左侧的导航项或模块进行编辑</p>
          </div>
        </div>
      </div>

      <!-- ===== Settings Mode ===== -->
      <div v-if="editorMode === 'settings'" class="settings-mode">
        <el-tabs v-model="settingsTab" type="border-card">
          <el-tab-pane label="基本设置" name="basic">
            <el-form :model="form" label-width="140px" style="max-width: 700px; padding: 20px 0">
              <el-form-item label="站点名称"><el-input v-model="form.name" placeholder="我的订阅站" /></el-form-item>
              <el-form-item label="站点描述"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
              <el-form-item label="开放注册"><el-switch v-model="registrationEnabled" /></el-form-item>
              <el-form-item label="门户访问路径"><el-input value="/p" disabled /></el-form-item>
              <el-form-item label="门户端口"><el-input v-model="portalPort" placeholder="留空使用面板端口" /></el-form-item>
            </el-form>
          </el-tab-pane>
          <el-tab-pane label="外观配置" name="appearance">
            <el-form :model="config" label-width="140px" style="max-width: 700px; padding: 20px 0">
              <el-form-item label="站点标题"><el-input v-model="config.site_name" placeholder="CoreFusion" /></el-form-item>
              <el-form-item label="Logo URL"><el-input v-model="config.logo_url" placeholder="https://..." /></el-form-item>
              <el-form-item label="主色调"><div class="color-row"><el-color-picker v-model="config.primary_color" /><el-input v-model="config.primary_color" style="width: 120px" /></div></el-form-item>
              <el-form-item label="背景色"><div class="color-row"><el-color-picker v-model="config.background_color" /><el-input v-model="config.background_color" style="width: 120px" /></div></el-form-item>
              <el-form-item label="卡片背景色"><div class="color-row"><el-color-picker v-model="config.card_background_color" /><el-input v-model="config.card_background_color" style="width: 120px" /></div></el-form-item>
              <el-form-item label="文字颜色"><div class="color-row"><el-color-picker v-model="config.text_color" /><el-input v-model="config.text_color" style="width: 120px" /></div></el-form-item>
              <el-form-item label="强调色"><div class="color-row"><el-color-picker v-model="config.accent_color" /><el-input v-model="config.accent_color" style="width: 120px" /></div></el-form-item>
              <el-form-item label="字体"><el-input v-model="config.font_family" /></el-form-item>
            </el-form>
          </el-tab-pane>
          <el-tab-pane label="API 参考" name="api">
            <div class="api-ref-container">
              <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px">
                <span style="font-size: 13px">以下为门户相关的完整 API 接口文档，含请求/响应示例。<code>admin_token</code> 为管理员登录 Token，<code>client_token</code> 为客户端登录 Token。</span>
              </el-alert>
              <el-collapse v-model="activeApiNames" class="api-collapse">
                <el-collapse-item v-for="(group, gi) in apiGroups" :key="gi" :name="gi" :title="group.title">
                  <div v-for="(api, ai) in group.apis" :key="ai" class="api-item">
                    <div class="api-item-header">
                      <span class="api-method" :class="'method-' + api.method.toLowerCase()">{{ api.method }}</span>
                      <code class="api-path">{{ api.path }}</code>
                      <span class="api-desc">{{ api.desc }}</span>
                    </div>
                    <div v-if="api.auth" class="api-auth">
                      <span class="api-auth-label">认证：</span>
                      <span class="api-auth-value">{{ api.auth }}</span>
                    </div>
                    <div v-if="api.requestBody" class="api-section-block">
                      <div class="api-block-label">请求示例</div>
                      <div class="api-copy-bar">
                        <el-button text size="small" :icon="CopyDocument" @click="copyText(api.requestBody)">复制</el-button>
                      </div>
                      <pre class="api-code-block"><code>{{ api.requestBody }}</code></pre>
                    </div>
                    <div v-if="api.response" class="api-section-block">
                      <div class="api-block-label">响应示例</div>
                      <pre class="api-code-block"><code>{{ api.response }}</code></pre>
                    </div>
                  </div>
                </el-collapse-item>
              </el-collapse>
            </div>
          </el-tab-pane>
        </el-tabs>
      </div>

      <!-- ===== Template Code Mode ===== -->
      <div v-if="editorMode === 'code'" class="settings-mode">
        <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px">
          <span style="font-size: 13px">
            <b>模板编辑</b> — 可直接修改门户的 HTML/CSS/JS 模板代码。修改后点击"保存"即生效。
            <br/>HTML 变量：<code v-pre>{{.SiteName}}</code> <code v-pre>{{.LogoURL}}</code> <code v-pre>{{.RegistrationEnabled}}</code>
            <code v-pre>{{registrationEnabledText}}</code> &nbsp;|&nbsp; CSS 变量：<code v-pre>{{.PrimaryColor}}</code>
            <code v-pre>{{.BackgroundColor}}</code> <code v-pre>{{.FontFamily}}</code> 等
          </span>
        </el-alert>
        <el-tabs v-model="codeTab" type="border-card">
          <el-tab-pane label="HTML 模板" name="html">
            <el-input v-model="form.html_content" type="textarea" :rows="28" style="font-family: 'SF Mono', 'Fira Code', Consolas, monospace; font-size: 12px" />
          </el-tab-pane>
          <el-tab-pane label="CSS 样式" name="css">
            <el-input v-model="form.css_content" type="textarea" :rows="28" style="font-family: 'SF Mono', 'Fira Code', Consolas, monospace; font-size: 12px" />
          </el-tab-pane>
          <el-tab-pane label="JavaScript" name="js">
            <el-input v-model="form.js_content" type="textarea" :rows="28" style="font-family: 'SF Mono', 'Fira Code', Consolas, monospace; font-size: 12px" />
          </el-tab-pane>
          <el-tab-pane label="重置与导出" name="advanced">
            <div style="padding: 20px 0; max-width: 600px">
              <el-alert type="warning" :closable="false" show-icon title="重置为默认模板" style="margin-bottom: 16px">
                此操作将覆盖当前所有自定义配置，恢复为系统内置默认模板。此操作不可撤销。
              </el-alert>
              <el-button type="danger" @click="handleReset">重置为默认模板</el-button>
            </div>
          </el-tab-pane>
        </el-tabs>
      </div>
    </div>

    <!-- Navigation Dialog -->
    <el-dialog v-model="navDialogVisible" title="添加导航项" width="500px">
      <el-form label-position="top" size="small">
        <el-form-item label="显示名称"><el-input v-model="newNav.label" placeholder="如：帮助中心" /></el-form-item>
        <el-form-item label="图标"><el-input v-model="newNav.icon" placeholder="如：📖" /></el-form-item>
        <el-form-item label="目标页面">
          <el-select v-model="newNav.target" style="width: 100%">
            <el-option label="概览" value="overview" /><el-option label="订阅" value="subscription" /><el-option label="安全" value="security" /><el-option label="日志" value="logs" />
          </el-select>
        </el-form-item>
        <el-form-item label="外部链接 (可选)"><el-input v-model="newNav.href" placeholder="https://..." /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="navDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmAddNav">添加</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { View, Check, Plus, Delete, ArrowUp, ArrowDown, CopyDocument } from '@element-plus/icons-vue'
import { getActivePortal, updateActivePortal, resetPortal } from '../api/portal'
import { marked } from 'marked'
import DOMPurify from 'dompurify'

// Configure marked
marked.setOptions({ breaks: true, gfm: true })

const loading = ref(false)
const saving = ref(false)
const editorMode = ref('builder')
const settingsTab = ref('basic')
const codeTab = ref('html')
const previewDevice = ref('desktop')
const hasUnsavedChanges = ref(false)
const registrationEnabled = ref(false)
const portalPort = ref('')
const navDialogVisible = ref(false)
const richTextEditRef = ref(null)
const mdEditRef = ref(null)
const noticeMdRef = ref(null)

const form = reactive({ name: '', description: '', html_content: '', css_content: '', js_content: '' })
const config = reactive({
  site_name: 'CoreFusion', logo_url: '', primary_color: '#667eea', background_color: '#f0f2f5',
  card_background_color: '#ffffff', text_color: '#303133', accent_color: '#67c23a',
  font_family: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
})

const navigation = ref([])
const modules = ref([])
const selectedNavId = ref('')
const selectedModuleId = ref('')

const newNav = reactive({ label: '', icon: '', target: 'overview', href: '', enabled: true, open_external: false })

const activeApiNames = ref([0])

const apiGroups = [
  {
    title: '门户管理接口（需管理员 Token）',
    apis: [
      {
        method: 'GET',
        path: '/api/v1/portal/active',
        desc: '获取当前活跃门户配置（含结构化导航、模块、主题字段）',
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
      { "id": "nav-overview", "label": "概览", "icon": "📊", "target": "overview", "href": "", "enabled": true, "open_external": false }
    ],
    "modules": [
      { "id": "mod-hero", "type": "hero", "title": "欢迎", "visible": true,
        "data": { "eyebrow": "CORE FUSION", "heading": "欢迎回来", "description": "..." },
        "style": { "variant": "primary", "align": "left" } }
    ]
  }
}`,
      },
      {
        method: 'PUT',
        path: '/api/v1/portal/active',
        desc: '更新门户配置（支持结构化字段：navigation / modules / theme）',
        auth: 'Authorization: Bearer <admin_token>',
        requestBody: `{
  "name": "MyPortal",
  "description": "我的订阅站",
  "navigation": [
    { "id": "nav-home", "label": "首页", "icon": "🏠", "target": "overview", "href": "", "enabled": true, "open_external": false }
  ],
  "modules": [
    { "id": "mod-hero", "type": "hero", "title": "欢迎", "visible": true,
      "data": { "eyebrow": "WELCOME", "heading": "欢迎", "description": "...", "button_text": "查看", "button_target": "subscription" },
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
        desc: '重置门户为内置默认模板',
        auth: 'Authorization: Bearer <admin_token>',
        requestBody: null,
        response: `{
  "code": 0,
  "data": { "name": "Default", "site_name": "CoreFusion", "navigation": [ ... ], "modules": [ ... ] }
}`,
      },
      {
        method: 'GET',
        path: '/api/v1/portal/templates',
        desc: '获取所有门户模板列表',
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
        desc: '创建新门户模板',
        auth: 'Authorization: Bearer <admin_token>',
        requestBody: `{
  "name": "自定义模板",
  "description": "我的自定义门户",
  "html_content": "<div>...</div>",
  "css_content": "body { ... }",
  "js_content": "console.log('hi')",
  "config": "{ ... }"
}`,
        response: `{
  "code": 0,
  "data": { "id": 2, "name": "自定义模板", "is_active": false }
}`,
      },
      {
        method: 'POST',
        path: '/api/v1/portal/templates/:id/activate',
        desc: '激活指定模板为当前门户',
        auth: 'Authorization: Bearer <admin_token>',
        requestBody: null,
        response: `{ "code": 0, "data": { "message": "activated", "id": 2 } }`,
      },
      {
        method: 'DELETE',
        path: '/api/v1/portal/templates/:id',
        desc: '删除指定模板（不能删除内置/活跃模板）',
        auth: 'Authorization: Bearer <admin_token>',
        requestBody: null,
        response: `{ "code": 0, "data": { "message": "deleted" } }`,
      },
    ],
  },
  {
    title: '公开接口（无需认证）',
    apis: [
      {
        method: 'GET',
        path: '/api/v1/public/settings',
        desc: '获取门户公开设置（是否开放注册等）',
        auth: '无需认证',
        requestBody: null,
        response: `{
  "code": 0,
  "data": { "registration_enabled": true }
}`,
      },
      {
        method: 'GET',
        path: '/api/v1/public/subscription-endpoints',
        desc: '获取可用订阅端点列表',
        auth: '无需认证',
        requestBody: null,
        response: `{
  "code": 0,
  "data": [
    { "id": 1, "name": "主站", "host": "example.com", "port": 8081 }
  ]
}`,
      },
    ],
  },
  {
    title: '客户端接口（公开 + 需客户端 Token）',
    apis: [
      {
        method: 'POST',
        path: '/api/v1/client-auth/register',
        desc: '用户注册（需开启开放注册）',
        auth: '无需认证',
        requestBody: `{
  "username": "user1",
  "password": "MyPass123!",
  "email": "user@example.com"
}`,
        response: `{
  "code": 0,
  "data": { "message": "注册成功" }
}`,
      },
      {
        method: 'POST',
        path: '/api/v1/client-auth/login',
        desc: '用户登录，获取客户端 Token',
        auth: '无需认证',
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
        desc: '获取当前登录用户信息',
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
        desc: '轮换订阅 Token（旧 Token 失效）',
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
        desc: '更新客户端安全设置',
        auth: 'Authorization: Bearer <client_token>',
        requestBody: `{
  "alert_on_traffic_limit": true,
  "traffic_alert_threshold": 90
}`,
        response: `{ "code": 0, "data": { "message": "设置已更新" } }`,
      },
      {
        method: 'POST',
        path: '/api/v1/client-auth/change-password',
        desc: '修改密码',
        auth: 'Authorization: Bearer <client_token>',
        requestBody: `{
  "old_password": "OldPass123!",
  "new_password": "NewPass456!"
}`,
        response: `{ "code": 0, "data": { "message": "密码修改成功" } }`,
      },
    ],
  },
]

function copyText(text) {
  navigator.clipboard.writeText(text).then(() => ElMessage.success('已复制到剪贴板')).catch(() => ElMessage.error('复制失败'))
}

const selectedNav = computed(() => navigation.value.find(n => n.id === selectedNavId.value))
const selectedModule = computed(() => modules.value.find(m => m.id === selectedModuleId.value))

const previewStyles = computed(() => ({
  '--pv-primary': config.primary_color || '#667eea',
  '--pv-bg': config.background_color || '#f0f2f5',
  '--pv-card-bg': config.card_background_color || '#fff',
  '--pv-text': config.text_color || '#303133',
  '--pv-accent': config.accent_color || '#67c23a',
  '--pv-font': config.font_family || 'sans-serif',
}))

const defaultNavigation = [
  { id: 'nav-overview', label: '概览', icon: '📊', target: 'overview', href: '', enabled: true, open_external: false },
  { id: 'nav-subscription', label: '订阅', icon: '📋', target: 'subscription', href: '', enabled: true, open_external: false },
  { id: 'nav-security', label: '安全', icon: '🔒', target: 'security', href: '', enabled: true, open_external: false },
  { id: 'nav-logs', label: '日志', icon: '📦', target: 'logs', href: '', enabled: true, open_external: false },
]

const defaultModules = [
  {
    id: 'mod-hero', type: 'hero', title: '欢迎', visible: true,
    data: { eyebrow: 'CORE FUSION', heading: '欢迎回来', description: '查看您的流量使用和订阅状态', button_text: '查看订阅', button_target: 'subscription' },
    style: { variant: 'primary', align: 'left' },
  },
  {
    id: 'md-notice', type: 'notice', title: '站点公告', visible: true,
    data: { title: '📢 公告', markdown: '欢迎使用 **CoreFusion** 订阅管理平台！\n\n- 支持 VLESS / VMess / Trojan / TUIC 等多种协议\n- 一键复制订阅链接\n- 流量使用实时查看\n' },
    style: { variant: 'info' },
  },
  {
    id: 'mod-stats', type: 'account_stats', title: '流量使用', visible: true,
    data: {}, style: { layout: 'grid' },
  },
  {
    id: 'mod-sub', type: 'subscription', title: '订阅信息', visible: true,
    data: { title: '订阅信息', description: '查看您的订阅链接和状态', button_text: '复制订阅链接' },
    style: {},
  },
]

function moduleTypeLabel(type) {
  return { hero: 'Hero', markdown: 'MD文档', rich_text: '富文本', notice: '公告', feature_cards: '功能卡片', account_stats: '数据', subscription: '订阅', custom_html: 'HTML' }[type] || type
}

function moduleStyle(mod) {
  const s = {}
  if (mod.style?.align === 'center') s.textAlign = 'center'
  return s
}

function renderMd(md) {
  if (!md) return ''
  const raw = marked.parse(md)
  return DOMPurify.sanitize(raw, { ADD_TAGS: ['input'], ADD_ATTR: ['type', 'checked', 'disabled'] })
}

function sanitizeHtml(html) {
  return DOMPurify.sanitize(html)
}

function genId(prefix) { return (prefix || 'id') + '-' + Math.random().toString(36).slice(2, 9) }

function selectNav(id) { selectedNavId.value = id; selectedModuleId.value = '' }
function selectModule(id) { selectedModuleId.value = id; selectedNavId.value = '' }

function openNavDialog() {
  Object.assign(newNav, { label: '', icon: '', target: 'overview', href: '', enabled: true, open_external: false })
  navDialogVisible.value = true
}
function confirmAddNav() {
  if (!newNav.label) { ElMessage.error('请输入显示名称'); return }
  navigation.value.push({ ...newNav, id: genId('nav') })
  navDialogVisible.value = false
  hasUnsavedChanges.value = true
}
function removeNav(idx) {
  ElMessageBox.confirm('确定删除此导航项？', '确认', { type: 'warning' }).then(() => {
    if (selectedNavId.value === navigation.value[idx].id) selectedNavId.value = ''
    navigation.value.splice(idx, 1)
    hasUnsavedChanges.value = true
  }).catch(() => {})
}

function addModule(type) {
  const defaults = {
    hero: { data: { eyebrow: '', heading: '新标题', description: '', button_text: '', button_target: 'subscription' }, style: { variant: 'primary', align: 'left' } },
    markdown: { data: { markdown: '## 标题\n\n在这里编写 Markdown 内容。\n\n- 支持 **粗体**、*斜体*\n- 支持 [链接](https://example.com)\n- 支持代码块\n\n```js\nconsole.log("hello")\n```\n' }, style: { align: 'left' } },
    rich_text: { data: { html: '<p>在此编辑富文本内容</p>' }, style: { align: 'left' } },
    notice: { data: { title: '公告', markdown: '公告正文，支持 **Markdown** 格式。' }, style: { variant: 'info' } },
    feature_cards: { data: { cards: [{ icon: '⭐', title: '功能一', description: '功能描述' }] }, style: {} },
    account_stats: { data: {}, style: { layout: 'grid' } },
    subscription: { data: { title: '订阅信息', description: '', button_text: '复制订阅链接' }, style: {} },
    custom_html: { data: { html: '<div>自定义内容</div>' }, style: {} },
  }
  const mod = { id: genId('mod'), type, title: moduleTypeLabel(type), visible: true, data: defaults[type].data, style: defaults[type].style }
  modules.value.push(mod)
  selectedModuleId.value = mod.id
  selectedNavId.value = ''
  hasUnsavedChanges.value = true
}

function duplicateModule(idx) {
  const orig = modules.value[idx]
  const copy = JSON.parse(JSON.stringify(orig))
  copy.id = genId('mod')
  copy.title = (orig.title || '') + ' (副本)'
  modules.value.splice(idx + 1, 0, copy)
  hasUnsavedChanges.value = true
}

function removeModule(idx) {
  ElMessageBox.confirm('确定删除此模块？', '确认', { type: 'warning' }).then(() => {
    if (selectedModuleId.value === modules.value[idx].id) selectedModuleId.value = ''
    modules.value.splice(idx, 1)
    hasUnsavedChanges.value = true
  }).catch(() => {})
}

function moveItem(list, idx, dir) {
  const ni = idx + dir
  if (ni < 0 || ni >= list.length) return
  const tmp = list[idx]; list[idx] = list[ni]; list[ni] = tmp
  hasUnsavedChanges.value = true
}

// Rich text editor commands
function execCmd(cmd, val) {
  document.execCommand(cmd, false, val)
  syncRichText()
}
function insertLink() {
  const url = window.prompt('输入链接地址')
  if (url) { document.execCommand('createLink', false, url); syncRichText() }
}
function onRichTextInput() { syncRichText() }
function syncRichText() {
  if (richTextEditRef.value && selectedModule.value) {
    selectedModule.value.data.html = richTextEditRef.value.innerHTML
    hasUnsavedChanges.value = true
  }
}

// Markdown editor toolbar
function insertMd(type) {
  const el = mdEditRef.value
  if (!el) return
  const start = el.selectionStart
  const end = el.selectionEnd
  const text = el.value
  const sel = text.substring(start, end)
  let insert = ''
  switch (type) {
    case 'heading': insert = `## ${sel || '标题'}`; break
    case 'bold': insert = `**${sel || '粗体文字'}**`; break
    case 'italic': insert = `*${sel || '斜体文字'}*`; break
    case 'strikethrough': insert = `~~${sel || '删除线文字'}~~`; break
    case 'ul': insert = `- ${sel || '列表项'}`; break
    case 'ol': insert = `1. ${sel || '列表项'}`; break
    case 'checklist': insert = `- [ ] ${sel || '待办项'}`; break
    case 'link': insert = `[${sel || '链接文字'}](https://example.com)`; break
    case 'image': insert = `![${sel || '图片描述'}](https://...)`; break
    case 'code': insert = `\`${sel || '代码'}\``; break
    case 'codeblock': insert = `\`\`\`\n${sel || '代码块'}\n\`\`\``; break
    case 'quote': insert = `> ${sel || '引用文字'}`; break
    case 'table': insert = `| 列1 | 列2 | 列3 |\n|------|------|------|\n| 内容 | 内容 | 内容 |`; break
    case 'hr': insert = `---`; break
  }
  el.value = text.substring(0, start) + insert + text.substring(end)
  el.selectionStart = el.selectionEnd = start + insert.length
  selectedModule.value.data.markdown = el.value
  hasUnsavedChanges.value = true
}

function onMdInput() { hasUnsavedChanges.value = true }

// Notice MD helpers
function insertNoticeMd(type) {
  const el = noticeMdRef.value
  if (!el) return
  const start = el.selectionStart
  const end = el.selectionEnd
  const text = el.value
  const sel = text.substring(start, end)
  let insert = ''
  switch (type) {
    case 'bold': insert = `**${sel || '粗体'}**`; break
    case 'italic': insert = `*${sel || '斜体'}*`; break
    case 'link': insert = `[${sel || '链接'}](https://...)`; break
    case 'code': insert = `\`${sel || '代码'}\``; break
  }
  el.value = text.substring(0, start) + insert + text.substring(end)
  el.selectionStart = el.selectionEnd = start + insert.length
  selectedModule.value.data.markdown = el.value
  hasUnsavedChanges.value = true
}
function onNoticeMdInput() { hasUnsavedChanges.value = true }

const applyData = (d) => {
  if (!d) return
  form.name = d.name || ''
  form.description = d.description || ''
  form.html_content = d.html_content || ''
  form.css_content = d.css_content || ''
  form.js_content = d.js_content || ''
  registrationEnabled.value = d.registration_enabled || false
  portalPort.value = d.portal_port || ''

  // API now returns structured fields directly
  config.site_name = d.site_name || 'CoreFusion'
  config.logo_url = d.logo_url || ''
  config.primary_color = d.primary_color || '#667eea'
  config.background_color = d.background_color || '#f0f2f5'
  config.card_background_color = d.card_background_color || '#ffffff'
  config.text_color = d.text_color || '#303133'
  config.accent_color = d.accent_color || '#67c23a'
  config.font_family = d.font_family || "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"

  // Fallback: if structured fields are empty but config string exists, parse it
  if ((!d.navigation || d.navigation.length === 0) && d.config) {
    try {
      const cfg = JSON.parse(d.config)
      Object.assign(config, cfg)
      navigation.value = Array.isArray(cfg.navigation) && cfg.navigation.length > 0 ? cfg.navigation : JSON.parse(JSON.stringify(defaultNavigation))
      modules.value = Array.isArray(cfg.modules) && cfg.modules.length > 0 ? cfg.modules : JSON.parse(JSON.stringify(defaultModules))
    } catch {
      navigation.value = JSON.parse(JSON.stringify(defaultNavigation))
      modules.value = JSON.parse(JSON.stringify(defaultModules))
    }
  } else {
    navigation.value = Array.isArray(d.navigation) && d.navigation.length > 0 ? d.navigation : JSON.parse(JSON.stringify(defaultNavigation))
    modules.value = Array.isArray(d.modules) && d.modules.length > 0 ? d.modules : JSON.parse(JSON.stringify(defaultModules))
  }
  hasUnsavedChanges.value = false
}

const fetchConfig = async () => {
  loading.value = true
  try {
    const data = await getActivePortal()
    applyData(data)
  } catch { ElMessage.error('加载门户配置失败') }
  finally { loading.value = false }
}

const handleSave = async () => {
  saving.value = true
  try {
    const data = {
      name: form.name, description: form.description,
      html_content: form.html_content, css_content: form.css_content, js_content: form.js_content,
      // Use structured fields — the backend will merge them into the config JSON
      navigation: navigation.value,
      modules: modules.value,
      theme: {
        site_name: config.site_name,
        logo_url: config.logo_url,
        primary_color: config.primary_color,
        background_color: config.background_color,
        card_background_color: config.card_background_color,
        text_color: config.text_color,
        accent_color: config.accent_color,
        font_family: config.font_family,
      },
      registration_enabled: registrationEnabled.value, portal_port: portalPort.value,
    }
    const res = await updateActivePortal(data)
    // Re-apply data from response to sync structured fields
    applyData(res)
    ElMessage.success('门户配置已保存')
    hasUnsavedChanges.value = false
  } catch (e) { ElMessage.error(e.response?.data?.message || '保存失败') }
  finally { saving.value = false }
}

const handlePreview = () => { window.open(window.location.origin + '/p', '_blank') }

const handleReset = async () => {
  try {
    await ElMessageBox.confirm('此操作将覆盖当前所有自定义配置，恢复为系统内置默认模板。此操作不可撤销！', '确认重置', { type: 'warning' })
    const res = await resetPortal()
    applyData(res)
    ElMessage.success('已重置为默认模板')
  } catch (e) { if (e !== 'cancel') ElMessage.error('重置失败') }
}

onMounted(() => { fetchConfig() })
</script>

<style scoped>
.portal-management { height: 100%; display: flex; flex-direction: column; overflow: hidden; }

.editor-topbar {
  display: flex; justify-content: space-between; align-items: center;
  padding: 8px 16px; background: #fff; border-bottom: 1px solid #e4e7ed;
  flex-shrink: 0; box-shadow: 0 1px 4px rgba(0,0,0,0.04);
}
.topbar-left { display: flex; align-items: center; gap: 8px; }
.portal-name { font-size: 16px; font-weight: 600; color: #303133; }
.topbar-right { display: flex; align-items: center; gap: 8px; }

.editor-body { flex: 1; overflow: hidden; }

/* ===== Builder Layout ===== */
.builder-layout { display: flex; height: 100%; gap: 1px; background: #e4e7ed; }

.col-structure { width: 280px; background: #fff; overflow-y: auto; display: flex; flex-direction: column; }
.col-canvas { flex: 1; background: #f5f7fa; display: flex; flex-direction: column; overflow: hidden; }
.col-properties { width: 420px; background: #fff; overflow-y: auto; }

.structure-section { padding: 12px; border-bottom: 1px solid #f0f0f0; }
.section-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.section-title { font-size: 13px; font-weight: 600; color: #303133; }

.nav-list, .module-list { display: flex; flex-direction: column; gap: 4px; }
.nav-item-row, .module-item-row {
  display: flex; justify-content: space-between; align-items: center;
  padding: 8px; border-radius: 6px; cursor: pointer; border: 1px solid transparent;
  transition: all 0.2s;
}
.nav-item-row:hover, .module-item-row:hover { background: #f5f7fa; }
.nav-item-row.selected, .module-item-row.selected { background: #ecf5ff; border-color: #409eff; }
.nav-item-row.disabled, .module-item-row.disabled { opacity: 0.5; }
.nav-item-info, .module-item-info { display: flex; align-items: center; gap: 6px; overflow: hidden; }
.nav-icon-emoji { font-size: 16px; }
.nav-label-text, .module-title-text { font-size: 13px; color: #303133; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.nav-item-actions, .module-item-actions { display: flex; align-items: center; gap: 2px; flex-shrink: 0; }

.module-type-badge {
  display: inline-block; padding: 1px 6px; border-radius: 4px; font-size: 11px; font-weight: 500;
}
.badge-hero { background: #e8f5e9; color: #4caf50; }
.badge-markdown { background: #e3f2fd; color: #1976d2; }
.badge-rich_text { background: #e8eaf6; color: #5c6bc0; }
.badge-notice { background: #fff3e0; color: #ff9800; }
.badge-feature_cards { background: #f3e5f5; color: #9c27b0; }
.badge-account_stats { background: #e0f7fa; color: #00bcd4; }
.badge-subscription { background: #fce4ec; color: #e91e63; }
.badge-custom_html { background: #efebe9; color: #795548; }

.empty-hint { text-align: center; padding: 16px; color: #909399; font-size: 13px; cursor: pointer; border: 1px dashed #dcdfe6; border-radius: 6px; }
.empty-hint:hover { color: #409eff; border-color: #409eff; }

/* ===== Canvas Preview ===== */
.canvas-toolbar { padding: 8px 16px; background: #fff; border-bottom: 1px solid #e4e7ed; flex-shrink: 0; }
.canvas-scroll { flex: 1; overflow-y: auto; padding: 20px; display: flex; justify-content: center; }
.device-desktop .portal-preview { width: 100%; max-width: 800px; }
.device-tablet .portal-preview { width: 768px; }
.device-mobile .portal-preview { width: 375px; }

.portal-preview {
  background: var(--pv-bg); border-radius: 16px; overflow: hidden;
  font-family: var(--pv-font); color: var(--pv-text);
  box-shadow: 0 4px 20px rgba(0,0,0,0.08); min-height: 600px;
  display: flex; flex-direction: column;
}

.preview-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 20px; background: var(--pv-card-bg);
  border-bottom: 1px solid rgba(0,0,0,0.06);
}
.preview-header-left { display: flex; align-items: center; gap: 8px; }
.preview-site-name { font-size: 16px; font-weight: 600; }
.preview-header-right { display: flex; gap: 12px; }
.preview-nav-item { font-size: 13px; color: #6c7293; cursor: pointer; }
.preview-nav-item:hover { color: var(--pv-primary); }

.preview-modules { flex: 1; padding: 16px; overflow-y: auto; }

.preview-module { margin-bottom: 16px; padding: 20px; background: var(--pv-card-bg); border-radius: 12px; }
.preview-hero { background: linear-gradient(135deg, var(--pv-primary), color-mix(in srgb, var(--pv-primary) 70%, #000)); color: #fff; }
.hero-eyebrow { font-size: 12px; text-transform: uppercase; opacity: 0.8; margin-bottom: 4px; }
.hero-heading { font-size: 24px; font-weight: 700; margin: 4px 0; }
.hero-desc { font-size: 14px; opacity: 0.9; margin-bottom: 12px; }
.hero-btn { padding: 8px 20px; border: none; border-radius: 8px; background: #fff; color: var(--pv-primary); font-weight: 600; cursor: pointer; }

/* Markdown Preview Styles */
.md-preview-content { font-size: 14px; line-height: 1.7; }
.md-preview-content :deep(h1) { font-size: 22px; font-weight: 700; margin: 16px 0 8px; padding-bottom: 6px; border-bottom: 1px solid rgba(0,0,0,0.08); }
.md-preview-content :deep(h2) { font-size: 18px; font-weight: 700; margin: 14px 0 6px; padding-bottom: 4px; border-bottom: 1px solid rgba(0,0,0,0.06); }
.md-preview-content :deep(h3) { font-size: 16px; font-weight: 600; margin: 12px 0 4px; }
.md-preview-content :deep(h4) { font-size: 14px; font-weight: 600; margin: 10px 0 4px; }
.md-preview-content :deep(p) { margin: 6px 0; }
.md-preview-content :deep(a) { color: var(--pv-primary); text-decoration: none; }
.md-preview-content :deep(a:hover) { text-decoration: underline; }
.md-preview-content :deep(strong) { font-weight: 600; }
.md-preview-content :deep(blockquote) { border-left: 3px solid var(--pv-primary); padding: 4px 12px; margin: 8px 0; color: #6c7293; background: var(--pv-bg); border-radius: 0 6px 6px 0; }
.md-preview-content :deep(code) { background: rgba(0,0,0,0.06); padding: 2px 6px; border-radius: 4px; font-size: 13px; font-family: 'SF Mono', 'Fira Code', Consolas, monospace; }
.md-preview-content :deep(pre) { background: #1e1e2e; color: #cdd6f4; padding: 14px; border-radius: 8px; overflow-x: auto; margin: 10px 0; }
.md-preview-content :deep(pre code) { background: none; padding: 0; color: inherit; }
.md-preview-content :deep(ul), .md-preview-content :deep(ol) { padding-left: 20px; margin: 6px 0; }
.md-preview-content :deep(li) { margin: 3px 0; }
.md-preview-content :deep(table) { border-collapse: collapse; width: 100%; margin: 10px 0; font-size: 13px; }
.md-preview-content :deep(th), .md-preview-content :deep(td) { border: 1px solid rgba(0,0,0,0.1); padding: 6px 10px; text-align: left; }
.md-preview-content :deep(th) { background: var(--pv-bg); font-weight: 600; }
.md-preview-content :deep(hr) { border: none; border-top: 1px solid rgba(0,0,0,0.1); margin: 16px 0; }
.md-preview-content :deep(img) { max-width: 100%; border-radius: 8px; }
.md-preview-content :deep(input[type="checkbox"]) { margin-right: 6px; }

.preview-notice { border-left: 4px solid; }
.notice-info { border-color: #409eff; background: #ecf5ff; }
.notice-success { border-color: #67c23a; background: #f0f9eb; }
.notice-warning { border-color: #e6a23c; background: #fdf6ec; }
.notice-error { border-color: #f56c6c; background: #fef0f0; }
.notice-title { font-weight: 600; margin-bottom: 4px; font-size: 14px; }
.notice-body { font-size: 13px; }

.preview-feature-cards .feature-cards-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 12px; }
.feature-card { padding: 12px; background: var(--pv-bg); border-radius: 8px; text-align: center; }
.feature-card-icon { font-size: 24px; display: block; }
.feature-card h4 { font-size: 13px; margin: 4px 0; }
.feature-card p { font-size: 12px; color: #909399; }

.preview-account-stats .stats-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
.stat-box { text-align: center; padding: 12px; background: var(--pv-bg); border-radius: 8px; }
.stat-num { font-size: 18px; font-weight: 700; display: block; }
.stat-label { font-size: 12px; color: #909399; }

.preview-subscription { text-align: center; }
.sub-desc { font-size: 13px; color: #6c7293; margin: 8px 0; }
.sub-btn { padding: 8px 20px; border: none; border-radius: 8px; background: var(--pv-primary); color: #fff; cursor: pointer; }

.module-section-title { font-size: 15px; font-weight: 600; margin-bottom: 12px; }

.custom-html-badge { display: inline-block; padding: 1px 8px; border-radius: 4px; background: #efebe9; color: #795548; font-size: 11px; font-weight: 600; margin-bottom: 8px; }

.preview-empty { text-align: center; padding: 40px; color: #909399; }

.preview-bottom-nav {
  display: flex; justify-content: space-around; padding: 8px 0;
  background: var(--pv-card-bg); border-top: 1px solid rgba(0,0,0,0.06);
}
.preview-bottom-nav-item { display: flex; flex-direction: column; align-items: center; gap: 2px; }
.preview-bottom-nav-item .nav-icon { font-size: 18px; }
.preview-bottom-nav-item .nav-label { font-size: 11px; color: #909399; }

/* ===== Properties Panel ===== */
.props-panel { padding: 16px; }
.props-title { font-size: 14px; font-weight: 600; margin-bottom: 16px; color: #303133; }
.props-empty { text-align: center; padding: 40px 20px; color: #909399; }
.props-empty-icon { font-size: 32px; display: block; margin-bottom: 8px; }

.card-edit-row { display: flex; gap: 4px; margin-bottom: 8px; }

/* ===== Markdown Editor ===== */
.md-editor-wrap { border: 1px solid #dcdfe6; border-radius: 6px; overflow: hidden; }
.md-editor-compact { margin-bottom: 8px; }
.md-editor-toolbar {
  display: flex; flex-wrap: wrap; gap: 2px; padding: 6px 8px;
  background: #fafafa; border-bottom: 1px solid #e4e7ed;
}
.md-editor-toolbar button {
  padding: 4px 8px; border: none; background: none; cursor: pointer;
  font-size: 13px; border-radius: 4px; transition: background 0.15s; color: #606266;
}
.md-editor-toolbar button:hover { background: #e4e7ed; }
.md-divider { width: 1px; background: #dcdfe6; margin: 2px 4px; }
.md-split { display: flex; min-height: 240px; }
.md-split-compact { min-height: 120px; }
.md-textarea {
  width: 50%; border: none; resize: none; padding: 12px;
  font-family: 'SF Mono', 'Fira Code', Consolas, monospace;
  font-size: 13px; line-height: 1.6; outline: none; color: #303133;
  background: #fff;
}
.md-textarea:focus { background: #fafcff; }
.md-preview-pane {
  width: 50%; padding: 12px; border-left: 1px solid #e4e7ed;
  overflow-y: auto; background: #fff;
}

/* ===== Rich Text Editor ===== */
.rich-text-editor { border: 1px solid #dcdfe6; border-radius: 6px; overflow: hidden; }
.rte-toolbar { display: flex; flex-wrap: wrap; gap: 2px; padding: 6px 8px; background: #f5f7fa; border-bottom: 1px solid #dcdfe6; }
.rte-toolbar button { padding: 4px 8px; border: none; background: none; cursor: pointer; font-size: 13px; border-radius: 4px; transition: background 0.2s; }
.rte-toolbar button:hover { background: #dcdfe6; }
.rte-divider { width: 1px; background: #dcdfe6; margin: 2px 4px; }
.rte-content { padding: 12px; min-height: 120px; font-size: 14px; line-height: 1.6; outline: none; }
.rte-content:focus { outline: 2px solid #409eff; }

/* ===== Settings/Code Mode ===== */
.settings-mode { padding: 16px; overflow-y: auto; height: 100%; }
.color-row { display: flex; align-items: center; gap: 8px; }
.api-section-title { font-size: 14px; font-weight: 600; color: #303133; margin-bottom: 12px; }

/* ===== API Reference ===== */
.api-ref-container { padding: 16px 0; max-width: 900px; }
.api-collapse { border: none; }
.api-collapse :deep(.el-collapse-item__header) { font-size: 14px; font-weight: 600; color: #303133; }
.api-collapse :deep(.el-collapse-item__content) { padding-bottom: 0; }
.api-item { border: 1px solid #ebeef5; border-radius: 8px; padding: 12px 16px; margin-bottom: 12px; background: #fafbfc; }
.api-item-header { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.api-method {
  display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 700;
  color: #fff; min-width: 52px; text-align: center;
}
.method-get { background: #409eff; }
.method-post { background: #67c23a; }
.method-put { background: #e6a23c; }
.method-delete { background: #f56c6c; }
.api-path { font-family: 'SF Mono', 'Fira Code', Consolas, monospace; font-size: 13px; color: #303133; font-weight: 600; }
.api-desc { font-size: 12px; color: #909399; }
.api-auth { font-size: 12px; color: #606266; margin-top: 6px; }
.api-auth-label { font-weight: 600; }
.api-auth-value { font-family: 'SF Mono', 'Fira Code', Consolas, monospace; color: #409eff; }
.api-section-block { margin-top: 8px; }
.api-block-label { font-size: 11px; font-weight: 600; color: #909399; margin-bottom: 4px; text-transform: uppercase; letter-spacing: 0.5px; }
.api-copy-bar { display: flex; justify-content: flex-end; margin: -4px 0 2px; }
.api-code-block {
  background: #1e1e2e; color: #cdd6f4; padding: 12px 16px; border-radius: 6px;
  overflow-x: auto; font-family: 'SF Mono', 'Fira Code', Consolas, monospace;
  font-size: 12px; line-height: 1.6; margin: 0;
}
.api-code-block code { color: inherit; }

/* ===== Responsive ===== */
@media (max-width: 1400px) {
  .col-structure { width: 240px; }
  .col-properties { width: 380px; }
}
@media (max-width: 1100px) {
  .builder-layout { flex-direction: column; }
  .col-structure, .col-properties { width: 100%; max-height: 250px; }
  .col-canvas { flex: 1; min-height: 300px; }
  .md-split { min-height: 180px; }
}
</style>
