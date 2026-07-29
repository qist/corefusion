<template>
  <div class="portal-management">
    <!-- Top Bar -->
    <div class="editor-topbar">
      <div class="topbar-left">
        <span class="portal-name">{{ form.name || t('portal.defaultName') }}</span>
        <el-tag v-if="hasUnsavedChanges" type="warning" size="small">{{ t('portal.unsaved') }}</el-tag>
        <el-tag v-else type="success" size="small">{{ t('portal.saved') }}</el-tag>
      </div>
      <div class="topbar-right">
        <el-button-group>
          <el-button :type="editorMode === 'builder' ? 'primary' : 'default'" size="small" @click="editorMode = 'builder'">{{ t('portal.builder') }}</el-button>
          <el-button :type="editorMode === 'lang' ? 'primary' : 'default'" size="small" @click="editorMode = 'lang'">{{ t('portal.lang') }}</el-button>
          <el-button :type="editorMode === 'settings' ? 'primary' : 'default'" size="small" @click="editorMode = 'settings'">{{ t('portal.settings') }}</el-button>
          <el-button :type="editorMode === 'code' ? 'primary' : 'default'" size="small" @click="editorMode = 'code'">{{ t('portal.templateEdit') }}</el-button>
        </el-button-group>
        <el-button size="small" :icon="View" @click="handlePreview">{{ t('portal.preview') }}</el-button>
        <el-button type="primary" size="small" :icon="Check" :loading="saving" @click="handleSave">{{ t('common.save') }}</el-button>
        <el-dropdown @command="handleCacheCommand" trigger="click">
          <el-button size="small">{{ t('portal.cache') }}<el-icon class="el-icon--right"><ArrowDown /></el-icon></el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="status">{{ t('portal.cacheStatus') }}{{ cacheStatus.cached ? t('portal.cached') : t('portal.notCached') }}</el-dropdown-item>
              <el-dropdown-item command="rebuild" divided>{{ t('portal.rebuildCache') }}</el-dropdown-item>
              <el-dropdown-item command="clear">{{ t('portal.clearCache') }}</el-dropdown-item>
              <el-dropdown-item command="dev" divided>{{ cacheStatus.dev_mode ? t('portal.switchToProduction') : t('portal.switchToDev') }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <div v-loading="loading" class="editor-body">
      <!-- ===== Builder Mode (Three-Column) ===== -->
      <div v-if="editorMode === 'builder'" class="builder-layout">
        <!-- Left Column: Page Structure -->
        <div class="col-structure">
          <!-- System Pages Section (Regions) -->
          <div class="structure-section">
            <div class="section-header">
              <span class="section-title">{{ t('portal.systemPages') }}</span>
            </div>
            <div class="nav-list">
              <div class="nav-item-row"
                   :class="{ selected: selectedRegionType === 'auth' }"
                   @click="selectRegion('auth')">
                <div class="nav-item-info">
                  <span class="nav-icon-emoji">🔐</span>
                  <span class="nav-label-text">{{ t('portal.loginPage') }}</span>
                </div>
              </div>
            </div>
          </div>

          <div class="structure-section">
            <div class="section-header">
              <span class="section-title">{{ t('portal.pageNav') }}</span>
              <el-button text size="small" :icon="Plus" @click="openNavDialog()" />
            </div>
            <div class="nav-list">
              <div
                v-for="(nav, idx) in navigation"
                :key="nav.id"
                class="nav-item-row"
                :class="{ selected: currentPageNavId === nav.id, disabled: !nav.enabled }"
                @click="selectNav(nav.id)"
              >
                <div class="nav-item-info">
                  <span class="nav-icon-emoji">{{ nav.icon || '🔗' }}</span>
                  <span class="nav-label-text">{{ nav.label }}</span>
                  <el-tag size="small" type="info" effect="plain">t('portal.moduleCount', { count: currentPageModuleCount(nav.target) })</el-tag>
                </div>
                <div class="nav-item-actions" @click.stop>
                  <el-button text size="small" :icon="ArrowUp" @click="moveItem(navigation, idx, -1)" :disabled="idx === 0" />
                  <el-button text size="small" :icon="ArrowDown" @click="moveItem(navigation, idx, 1)" :disabled="idx === navigation.length - 1" />
                  <el-switch v-model="nav.enabled" size="small" />
                  <el-button text size="small" type="danger" :icon="Delete" @click="removeNav(idx)" />
                </div>
              </div>
              <div v-if="navigation.length === 0" class="empty-hint" @click="openNavDialog()">
                t('portal.addNav')
              </div>
            </div>
          </div>

          <div class="structure-section structure-modules">
            <div class="section-header">
              <span class="section-title">
                {{ currentNav ? currentNav.label + ' 模块' : '内容模块' }}
              </span>
              <el-dropdown @command="addModule" trigger="click" :disabled="!currentNav">
                <el-button text size="small" :icon="Plus" :disabled="!currentNav">{{ t('portal.addModule') }}</el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item disabled style="font-size:11px;color:#909399;font-weight:600">— {{ t('portal.catContent') }} —</el-dropdown-item>
                    <el-dropdown-item command="hero">🚀 {{ t('portal.modHero') }}</el-dropdown-item>
                    <el-dropdown-item command="markdown">📝 {{ t('portal.modMarkdown') }}</el-dropdown-item>
                    <el-dropdown-item command="notice">📢 {{ t('portal.modNotice') }}</el-dropdown-item>
                    <el-dropdown-item command="freeform">🧩 {{ t('portal.modFreeform') }}</el-dropdown-item>
                    <el-dropdown-item command="tutorial_cards">📚 {{ t('portal.modTutorialCards') }}</el-dropdown-item>
                    <el-dropdown-item command="article_list">📖 {{ t('portal.modArticleList') }}</el-dropdown-item>
                    <el-dropdown-item disabled style="font-size:11px;color:#909399;font-weight:600" divided>— {{ t('portal.catAccount') }} —</el-dropdown-item>
                    <el-dropdown-item command="account_info">👤 {{ t('portal.modAccountInfo') }}</el-dropdown-item>
                    <el-dropdown-item command="traffic_ring">📊 {{ t('portal.modTrafficRing') }}</el-dropdown-item>
                    <el-dropdown-item command="subscription_detail">🔗 {{ t('portal.modSubDetail') }}</el-dropdown-item>
                    <el-dropdown-item command="client_download">💻 {{ t('portal.modClientDownload') }}</el-dropdown-item>
                    <el-dropdown-item command="streaming_status">🎬 {{ t('portal.modStreamingStatus') }}</el-dropdown-item>
                    <el-dropdown-item command="shared_accounts">🔑 {{ t('portal.modSharedAccounts') }}</el-dropdown-item>
                    <el-dropdown-item disabled style="font-size:11px;color:#909399;font-weight:600" divided>— {{ t('portal.catBusiness') }} —</el-dropdown-item>
                    <el-dropdown-item command="plan_list">📦 {{ t('portal.modPlanList') }}</el-dropdown-item>
                    <el-dropdown-item command="charge_panel">💳 {{ t('portal.modChargePanel') }}</el-dropdown-item>
                    <el-dropdown-item command="balance_display">💰 {{ t('portal.modBalanceDisplay') }}</el-dropdown-item>
                    <el-dropdown-item command="order_list">🧾 {{ t('portal.modOrderList') }}</el-dropdown-item>
                    <el-dropdown-item command="referral">🎁 {{ t('portal.modReferral') }}</el-dropdown-item>
                    <el-dropdown-item disabled style="font-size:11px;color:#909399;font-weight:600" divided>— {{ t('portal.catSystem') }} —</el-dropdown-item>
                    <el-dropdown-item command="node_list">📋 {{ t('portal.modNodeList') }}</el-dropdown-item>
                    <el-dropdown-item command="ticket_list">🎫 {{ t('portal.modTicketList') }}</el-dropdown-item>
                    <el-dropdown-item command="security_settings">🔒 {{ t('portal.modSecuritySettings') }}</el-dropdown-item>
                    <el-dropdown-item command="log_packages">📦 {{ t('portal.modLogPackages') }}</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
            <div class="module-list">
              <div
                v-for="(mod, idx) in currentPageModules"
                :key="mod.id"
                class="module-item-row"
                :class="{ selected: selectedModuleId === mod.id, disabled: !mod.visible }"
                @click="selectModule(mod.id)"
              >
                <div class="module-item-info">
                  <span class="module-type-badge" :class="'badge-' + mod.type">{{ moduleTypeLabel(mod.type) }}</span>
                  <span class="module-title-text">{{ mod.title || t('portal.noTitle') }}</span>
                </div>
                <div class="module-item-actions" @click.stop>
                  <el-button text size="small" :icon="ArrowUp" @click="movePageModule(idx, -1)" :disabled="idx === 0" />
                  <el-button text size="small" :icon="ArrowDown" @click="movePageModule(idx, 1)" :disabled="idx === currentPageModules.length - 1" />
                  <el-switch v-model="mod.visible" size="small" />
                  <el-button text size="small" :icon="CopyDocument" @click="duplicateModuleByIdx(idx)" />
                  <el-button text size="small" type="danger" :icon="Delete" @click="removeModuleByIdx(idx)" />
                </div>
              </div>
              <div v-if="!currentNav" class="empty-hint">
                t('portal.clickNavHint')
              </div>
              <div v-else-if="currentPageModules.length === 0" class="empty-hint" @click="addModule('hero')">
                t('portal.addFirstModule', { name: currentNav.label })
              </div>
            </div>
          </div>
        </div>

        <!-- Center Column: Code Editor or Visual Preview -->
        <div class="col-canvas">
          <!-- Region Editor Toolbar (when a region is selected) -->
          <div v-if="selectedRegion" class="canvas-toolbar">
            <div class="canvas-toolbar-left">
              <el-radio-group v-model="regionViewMode" size="small">
                <el-radio-button value="preview">{{ t('portal.visualPreview') }}</el-radio-button>
                <el-radio-button value="html">HTML</el-radio-button>
                <el-radio-button value="css">CSS</el-radio-button>
                <el-radio-button value="js">JS</el-radio-button>
              </el-radio-group>
            </div>
            <div class="canvas-toolbar-right">
              <el-radio-group v-if="regionViewMode === 'preview'" v-model="previewDevice" size="small">
                <el-radio-button value="desktop">{{ t('portal.deviceDesktop') }}</el-radio-button>
                <el-radio-button value="tablet">{{ t('portal.deviceTablet') }}</el-radio-button>
                <el-radio-button value="mobile">{{ t('portal.deviceMobile') }}</el-radio-button>
              </el-radio-group>
            </div>
          </div>

          <!-- Region Preview Mode (iframe) -->
          <div v-if="selectedRegion && regionViewMode === 'preview'" class="canvas-scroll">
            <iframe ref="previewFrame" :src="previewUrl" class="preview-iframe" />
          </div>

          <!-- Region HTML/CSS/JS Editor -->
          <div v-if="selectedRegion && regionViewMode !== 'preview'" class="canvas-code-editor">
            <CodeEditor
              v-model="selectedRegionSource[regionViewMode]"
              :language="regionViewMode === 'html' ? 'html' : regionViewMode === 'css' ? 'css' : 'javascript'"
              @change="onCodeChange"
            />
          </div>

          <!-- Toolbar: view mode toggle + device buttons -->
          <div v-if="selectedModule" class="canvas-toolbar">
            <div class="canvas-toolbar-left">
              <el-radio-group v-model="centerViewMode" size="small">
                <el-radio-button v-if="hasContentEditor" value="editor">{{ t('portal.contentEditor') }}</el-radio-button>
                <el-radio-button value="preview">{{ t('portal.visualPreview') }}</el-radio-button>
                <el-radio-button value="page-preview">{{ t('portal.pagePreview') }}</el-radio-button>
                <el-radio-button value="html">HTML</el-radio-button>
                <el-radio-button value="css">CSS</el-radio-button>
                <el-radio-button value="js">JS</el-radio-button>
              </el-radio-group>
            </div>
            <div class="canvas-toolbar-right">
              <el-radio-group v-if="centerViewMode === 'preview' || centerViewMode === 'editor'" v-model="previewDevice" size="small">
                <el-radio-button value="desktop">{{ t('portal.deviceDesktop') }}</el-radio-button>
                <el-radio-button value="tablet">{{ t('portal.deviceTablet') }}</el-radio-button>
                <el-radio-button value="mobile">{{ t('portal.deviceMobile') }}</el-radio-button>
              </el-radio-group>
            </div>
          </div>

          <!-- Content Editor Mode (markdown, rich_text, notice, custom_html) -->
          <div v-if="selectedModule && centerViewMode === 'editor' && hasContentEditor" class="canvas-code-editor">
            <!-- Markdown Editor -->
            <template v-if="selectedModule.type === 'markdown'">
              <div class="md-editor-wrap md-editor-full">
                <div class="md-editor-toolbar">
                  <button @click="insertMd('heading')" :title="t('portal.mdHeading')">H</button>
                  <button @click="insertMd('bold')" :title="t('portal.mdBold')"><b>B</b></button>
                  <button @click="insertMd('italic')" :title="t('portal.mdItalic')"><i>I</i></button>
                  <button @click="insertMd('strikethrough')" :title="t('portal.mdStrikethrough')"><s>S</s></button>
                  <span class="md-divider"></span>
                  <button @click="insertMd('ul')" :title="t('portal.mdUnorderedList')">•</button>
                  <button @click="insertMd('ol')" :title="t('portal.mdOrderedList')">1.</button>
                  <button @click="insertMd('checklist')" :title="t('portal.mdChecklist')">☑</button>
                  <span class="md-divider"></span>
                  <button @click="insertMd('link')" :title="t('portal.mdLink')">🔗</button>
                  <button @click="insertMd('image')" :title="t('portal.mdImage')">🖼</button>
                  <button @click="insertMd('code')" :title="t('portal.mdCode')">&lt;/&gt;</button>
                  <button @click="insertMd('codeblock')" :title="t('portal.mdCodeBlock')">{ }</button>
                  <button @click="insertMd('quote')" :title="t('portal.mdQuote')">❝</button>
                  <button @click="insertMd('table')" :title="t('portal.mdTable')">⊞</button>
                  <button @click="insertMd('hr')" :title="t('portal.mdHr')">—</button>
                </div>
                <div class="md-split md-split-full">
                  <div class="md-editor-pane">
                    <CodeEditor
                      v-model="selectedModule.data.markdown"
                      language="markdown"
                      @mount="onMdEditorMount"
                      @change="onCodeChange"
                    />
                  </div>
                  <div class="md-preview-pane md-preview-content" :class="'device-' + previewDevice" v-html="renderMd(selectedModule.data.markdown || '')"></div>
                </div>
              </div>
            </template>

            <!-- Notice Editor -->
            <template v-else-if="selectedModule.type === 'notice'">
              <div class="notice-editor-wrap">
                <el-form-item :label="t('portal.noticeTitle')" style="margin-bottom: 8px"><el-input v-model="selectedModule.data.title" :placeholder="t('portal.noticeTitle')" /></el-form-item>
                <div class="md-editor-wrap md-editor-full">
                  <div class="md-editor-toolbar">
                    <button @click="insertNoticeMd('heading')" :title="t('portal.mdHeading')">H</button>
                    <button @click="insertNoticeMd('bold')" :title="t('portal.mdBold')"><b>B</b></button>
                    <button @click="insertNoticeMd('italic')" :title="t('portal.mdItalic')"><i>I</i></button>
                    <button @click="insertNoticeMd('link')" :title="t('portal.mdLink')">🔗</button>
                    <button @click="insertNoticeMd('code')" :title="t('portal.mdCode')">&lt;/&gt;</button>
                  </div>
                  <div class="md-split md-split-full">
                    <div class="md-editor-pane">
                      <CodeEditor
                        v-model="selectedModule.data.markdown"
                        language="markdown"
                        @mount="onNoticeMdEditorMount"
                        @change="onCodeChange"
                      />
                    </div>
                    <div class="md-preview-pane md-preview-content" :class="'device-' + previewDevice" v-html="renderMd(selectedModule.data.markdown || '')"></div>
                  </div>
                </div>
              </div>
            </template>

            <!-- Rich Text Editor -->
            <template v-else-if="selectedModule.type === 'rich_text'">
              <div class="rte-editor-wrap">
                <div class="rte-toolbar">
                  <button @click="execCmd('formatBlock', '<h2>')" :title="t('portal.rteHeading1')">H2</button>
                  <button @click="execCmd('formatBlock', '<h3>')" :title="t('portal.rteHeading2')">H3</button>
                  <button @click="execCmd('formatBlock', '<p>')" :title="t('portal.rteParagraph')">P</button>
                  <span class="rte-divider"></span>
                  <button @click="execCmd('bold')" :title="t('portal.mdBold')"><b>B</b></button>
                  <button @click="execCmd('italic')" :title="t('portal.mdItalic')"><i>I</i></button>
                  <button @click="execCmd('underline')" :title="t('portal.rteUnderline')"><u>U</u></button>
                  <button @click="execCmd('strikeThrough')" :title="t('portal.mdStrikethrough')"><s>S</s></button>
                  <span class="rte-divider"></span>
                  <button @click="execCmd('insertUnorderedList')" :title="t('portal.mdUnorderedList')">•</button>
                  <button @click="execCmd('insertOrderedList')" :title="t('portal.mdOrderedList')">1.</button>
                  <button @click="execCmd('formatBlock', '<blockquote>')" :title="t('portal.mdQuote')">❝</button>
                  <span class="rte-divider"></span>
                  <button @click="insertLink" :title="t('portal.rteHyperlink')">🔗</button>
                  <button @click="execCmd('removeFormat')" :title="t('portal.rteClearFormat')">✕</button>
                </div>
                <div class="rte-split">
                  <div ref="richTextEditRef" class="rte-content rte-content-full" contenteditable="true" @input="onRichTextInput" v-html="selectedModule.data.html || ''"></div>
                  <div class="rte-preview md-preview-content" :class="'device-' + previewDevice" v-html="sanitizeHtml(selectedModule.data.html || '')"></div>
                </div>
              </div>
            </template>

            <!-- Custom HTML Editor -->
            <template v-else-if="selectedModule.type === 'custom_html'">
              <div class="html-editor-wrap">
                <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 8px">
                  <span style="font-size: 12px">{{ t('portal.customHtmlWarning') }}</span>
                </el-alert>
                <div class="html-split">
                  <div class="html-editor-pane">
                    <CodeEditor
                      v-model="selectedModule.data.html"
                      language="html"
                      @change="onCodeChange"
                    />
                  </div>
                  <div class="html-preview md-preview-content" :class="'device-' + previewDevice" v-html="sanitizeHtml(selectedModule.data.html || '')"></div>
                </div>
              </div>
            </template>

            <!-- Freeform Editor (HTML + CSS + JS) -->
            <template v-else-if="selectedModule.type === 'freeform'">
              <div class="freeform-editor-wrap">
                <el-tabs v-model="freeformTab" type="border-card" class="freeform-tabs">
                  <el-tab-pane :label="t('portal.freeformHtmlTab')" name="html">
                    <div class="freeform-editor-pane">
                      <CodeEditor
                        v-model="selectedModule.data.html"
                        language="html"
                        @change="onCodeChange"
                      />
                    </div>
                  </el-tab-pane>
                  <el-tab-pane label="CSS" name="css">
                    <div class="freeform-editor-pane">
                      <CodeEditor
                        v-model="selectedModule.data.css"
                        language="css"
                        @change="onCodeChange"
                      />
                    </div>
                  </el-tab-pane>
                  <el-tab-pane :label="t('portal.freeformJsTab')" name="js">
                    <div class="freeform-editor-pane">
                      <CodeEditor
                        v-model="selectedModule.data.js"
                        language="javascript"
                        @change="onCodeChange"
                      />
                    </div>
                    <div class="freeform-api-ref">
                      <el-collapse>
                        <el-collapse-item :title="t('portal.ctxApiRef')">
                          <pre>{{ t('portal.ctxApiRefBody') }}</pre>
                        </el-collapse-item>
                      </el-collapse>
                    </div>
                  </el-tab-pane>
                </el-tabs>
                <el-alert type="info" :closable="false" show-icon style="margin-top: 8px">
                  <span style="font-size: 12px">{{ t('portal.jsHint') }}</span>
                </el-alert>
              </div>
            </template>
          </div>

          <!-- CSS Editor Mode (available for ALL module types) -->
          <div v-if="selectedModule && centerViewMode === 'css'" class="canvas-css-editor">
            <div class="css-editor-info">
              <el-alert type="info" :closable="false" show-icon style="margin-bottom: 8px; border-radius: 6px">
                <span style="font-size: 12px">
                  {{ t('portal.cssHint1') }}<b>{{ selectedModule.title || moduleTypeLabel(selectedModule.type) }}</b>{{ t('portal.cssHint2') }}
                </span>
              </el-alert>
            </div>
            <div class="css-editor-split">
              <div class="css-editor-pane">
                <CodeEditor
                  v-model="selectedModule.style.custom_css"
                  language="css"
                  @change="onCodeChange"
                />
              </div>
              <div class="css-preview-pane" :class="'device-' + previewDevice">
                <div class="portal-preview" :style="previewStyles">
                  <div class="preview-header">
                    <div class="preview-header-left">
                      <span class="preview-site-name">{{ config.site_name || 'CoreFusion' }}</span>
                    </div>
                  </div>
                  <div class="preview-modules">
                    <!-- Module preview using user-edited HTML/CSS/JS from database -->
                    <div :id="'preview-mod-' + selectedModule.id" class="module-wrap-preview">
                      <!-- User-edited CSS -->
                      <style v-if="selectedModule.data?.css" v-html="selectedModule.data.css"></style>
                      <style v-if="selectedModule.style?.custom_css" v-html="scopeCss(selectedModule.style.custom_css, selectedModule.id)"></style>
                      
                      <!-- Dynamic component from user-edited HTML/JS -->
                      <component :is="dynamicModuleComponent" v-if="dynamicModuleComponent" />
                      
                      <!-- Empty state if no code -->
                      <div v-else class="preview-module" style="text-align:center;padding:40px;color:#909399">
                        <p>{{ t('portal.noCustomCode') }}</p>
                        <p style="font-size:12px;margin-top:8px">{{ t('portal.addCodeHint') }}</p>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Page Preview Mode (iframe — like login module) -->
          <div v-if="selectedModule && centerViewMode === 'page-preview'" class="canvas-scroll">
            <iframe :src="modulePreviewUrl" class="preview-iframe" />
          </div>

          <!-- HTML Editor Mode -->
          <div v-if="selectedModule && centerViewMode === 'html'" class="canvas-code-editor">
            <el-alert type="info" :closable="false" show-icon style="margin-bottom: 8px">
              <span style="font-size: 12px">{{ t('portal.htmlEditorHint') }}</span>
            </el-alert>
            <CodeEditor
              v-model="selectedModule.data.html"
              language="html"
              @change="onCodeChange"
            />
          </div>

          <!-- JS Editor Mode -->
          <div v-if="selectedModule && centerViewMode === 'js'" class="canvas-code-editor">
            <CodeEditor
              v-model="selectedModule.data.js"
              language="javascript"
              @change="onCodeChange"
            />
            <div class="freeform-api-ref" style="padding: 8px 12px">
              <el-collapse>
                <el-collapse-item :title="t('portal.ctxApiRef')">
                  <pre>{{ t('portal.ctxApiRefBody') }}</pre>
                </el-collapse-item>
              </el-collapse>
            </div>
          </div>

          <!-- Visual Preview Mode -->
          <div v-if="!selectedModule || (centerViewMode === 'preview')" class="canvas-scroll">
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

              <!-- Page Title -->
              <div v-if="currentNav" class="preview-page-title">
                <span class="page-title-icon">{{ currentNav.icon || '📄' }}</span>
                <span class="page-title-text">{{ currentNav.label }}</span>
              </div>

              <!-- Module Previews - Using Dynamic Components from Database -->
              <div class="preview-modules">
                <template v-for="mod in currentPageModules.filter(m => m.visible)" :key="mod.id">
                  <div class="preview-module" :id="'preview-mod-' + mod.id">
                    <!-- User-edited CSS -->
                    <style v-if="mod.data?.css" v-html="mod.data.css"></style>
                    <!-- Dynamic Component -->
                    <component :is="getDynamicComponent(mod)" v-if="getDynamicComponent(mod)" />
                    <!-- Empty State -->
                    <div v-else style="text-align:center;padding:40px;color:#909399">
                      <p>{{ mod.title || moduleTypeLabel(mod.type) }}</p>
                      <p style="font-size:12px;margin-top:8px">{{ t('portal.noCustomCode') }}</p>
                    </div>
                  </div>
                </template>

                <div v-if="!currentNav" class="preview-empty">
                  <span style="font-size: 40px">👈</span>
                  <p>{{ t('portal.selectPageHint') }}</p>
                </div>
                <div v-else-if="currentPageModules.filter(m => m.visible).length === 0" class="preview-empty">
                  <span style="font-size: 40px">📋</span>
                  <p>{{ t('portal.pageNoModules', { name: currentNav.label }) }}</p>
                  <el-button type="primary" size="small" :icon="Plus" @click="addModule('hero')" style="margin-top: 8px">{{ t('portal.addModule') }}</el-button>
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
          <!-- Region Properties -->
          <div v-if="selectedRegion" class="props-panel">
            <h3 class="props-title">{{ t('portal.editLoginPage') }}</h3>
            <el-form label-position="top" size="small">
              <el-divider content-position="left">{{ t('portal.basicSettings') }}</el-divider>
              <el-form-item :label="t('portal.subtitle')">
                <el-input v-model="selectedRegion.data.subtitle" @input="onCodeChange" />
              </el-form-item>
              <el-form-item :label="t('portal.logoEmoji')">
                <el-input v-model="selectedRegion.data.logo_emoji" @input="onCodeChange" />
              </el-form-item>

              <el-divider content-position="left">{{ t('portal.formText') }}</el-divider>
              <el-form-item :label="t('portal.loginLabel')">
                <el-input v-model="selectedRegion.data.login_label" @input="onCodeChange" />
              </el-form-item>
              <el-form-item :label="t('portal.registerLabel')">
                <el-input v-model="selectedRegion.data.register_label" @input="onCodeChange" />
              </el-form-item>
              <el-form-item :label="t('portal.loginBtn')">
                <el-input v-model="selectedRegion.data.login_btn_text" @input="onCodeChange" />
              </el-form-item>
              <el-form-item :label="t('portal.registerBtn')">
                <el-input v-model="selectedRegion.data.register_btn_text" @input="onCodeChange" />
              </el-form-item>
              <el-form-item :label="t('portal.emailPlaceholder')">
                <el-input v-model="selectedRegion.data.email_placeholder" @input="onCodeChange" />
              </el-form-item>
              <el-form-item :label="t('portal.passwordPlaceholder')">
                <el-input v-model="selectedRegion.data.password_placeholder" @input="onCodeChange" />
              </el-form-item>
              <el-form-item :label="t('portal.invitePlaceholder')">
                <el-input v-model="selectedRegion.data.invite_placeholder" @input="onCodeChange" />
              </el-form-item>

              <el-divider content-position="left">{{ t('portal.footerArea') }}</el-divider>
              <el-form-item :label="t('portal.footerText')">
                <el-input v-model="selectedRegion.data.footer_text" @input="onCodeChange" />
              </el-form-item>

              <el-divider content-position="left">{{ t('portal.jsCodingSpec') }}</el-divider>
              <el-alert type="info" :closable="false" show-icon>
                <span style="font-size: 11px">
                  <b>{{ t('portal.jsObjectLiteral') }}</b>：<code>{ customTitle: t('portal.jsExampleCustomTitle') }</code><br>
                  <b>{{ t('portal.jsReturnStatement') }}</b>：<code>return { customTitle: t('portal.jsExampleCustomTitle') }</code><br>
                  <b>{{ t('portal.jsFullFunction') }}</b>：<code>console.log('init'); return {}</code><br>
                  <b>{{ t('portal.jsAvailableCtx') }}</b>：loginForm, registerForm, authTab, loading, handleLogin, handleRegister, t, data, ElMessage
                </span>
              </el-alert>
            </el-form>
          </div>

          <!-- Navigation Properties -->
          <div v-else-if="selectedNavId && selectedNav" class="props-panel">
            <h3 class="props-title">{{ t('portal.editNav') }}</h3>
            <el-form label-position="top" size="small">
              <el-form-item :label="t('portal.displayName')"><el-input v-model="selectedNav.label" :placeholder="t('portal.displayNamePlaceholder')" /></el-form-item>
              <el-form-item :label="t('portal.navIcon')"><el-input v-model="selectedNav.icon" :placeholder="t('portal.navIconPlaceholder')" /></el-form-item>
              <el-form-item :label="t('portal.pageTarget')"><el-input v-model="selectedNav.target" placeholder="auto" disabled /></el-form-item>
              <el-form-item :label="t('portal.externalLink')"><el-input v-model="selectedNav.href" :placeholder="t('portal.externalLinkPlaceholder')" /></el-form-item>
              <el-form-item :label="t('portal.openInNewWindow')"><el-switch v-model="selectedNav.open_external" /></el-form-item>
              <el-form-item :label="t('portal.showNav')"><el-switch v-model="selectedNav.enabled" /></el-form-item>
            </el-form>
          </div>

          <!-- Module Properties -->
          <div v-else-if="selectedModuleId && selectedModule" class="props-panel">
            <h3 class="props-title">{{ t('portal.editModule', { type: moduleTypeLabel(selectedModule.type) }) }}</h3>
            <el-form label-position="top" size="small">
              <el-form-item :label="t('portal.moduleTitle')"><el-input v-model="selectedModule.title" :placeholder="t('portal.moduleTitle')" /></el-form-item>
              <el-form-item :label="t('portal.showModule')"><el-switch v-model="selectedModule.visible" /></el-form-item>

              <!-- Hero Properties -->
              <template v-if="selectedModule.type === 'hero'">
                <el-form-item :label="t('portal.heroEyebrow')"><el-input v-model="selectedModule.data.eyebrow" :placeholder="t('portal.heroEyebrowPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.heroHeading')"><el-input v-model="selectedModule.data.heading" :placeholder="t('portal.heroHeadingPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.heroDescription')"><el-input v-model="selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
                <el-form-item :label="t('portal.heroButtonText')"><el-input v-model="selectedModule.data.button_text" :placeholder="t('portal.heroButtonPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.heroButtonTarget')">
                  <el-select v-model="selectedModule.data.button_target" style="width: 100%">
                    <el-option :label="t('portal.targetOverview')" value="overview" /><el-option :label="t('portal.targetSubscription')" value="subscription" /><el-option :label="t('portal.targetSecurity')" value="security" /><el-option :label="t('portal.targetLogs')" value="logs" />
                  </el-select>
                </el-form-item>
                <el-form-item :label="t('portal.alignment')">
                  <el-radio-group v-model="selectedModule.style.align">
                    <el-radio value="left">{{ t('portal.alignLeft') }}</el-radio><el-radio value="center">{{ t('portal.alignCenter') }}</el-radio>
                  </el-radio-group>
                </el-form-item>
                <el-form-item :label="t('portal.styleVariant')">
                  <el-select v-model="selectedModule.style.variant" style="width: 100%">
                    <el-option :label="t('portal.variantPrimary')" value="primary" /><el-option :label="t('portal.variantGradient')" value="gradient" /><el-option :label="t('portal.variantMinimal')" value="minimal" />
                  </el-select>
                </el-form-item>
              </template>

              <!-- Markdown Properties (editor is in center column) -->
              <template v-if="selectedModule.type === 'markdown'">
                <el-form-item :label="t('portal.alignment')">
                  <el-radio-group v-model="selectedModule.style.align">
                    <el-radio value="left">{{ t('portal.alignLeft') }}</el-radio><el-radio value="center">{{ t('portal.alignCenter') }}</el-radio>
                  </el-radio-group>
                </el-form-item>
              </template>

              <!-- Notice Properties (editor is in center column) -->
              <template v-if="selectedModule.type === 'notice'">
                <el-form-item :label="t('portal.statusColor')">
                  <el-select v-model="selectedModule.style.variant" style="width: 100%">
                    <el-option :label="t('portal.statusInfo')" value="info" /><el-option :label="t('portal.statusSuccess')" value="success" /><el-option :label="t('portal.statusWarning')" value="warning" /><el-option :label="t('portal.statusError')" value="error" />
                  </el-select>
                </el-form-item>
                <el-divider content-position="left">{{ t('portal.popupSettings') }}</el-divider>
                <el-form-item :label="t('portal.displayMode')">
                  <el-radio-group v-model="selectedModule.data.display_mode">
                    <el-radio value="inline">{{ t('portal.modeInline') }}</el-radio>
                    <el-radio value="popup">{{ t('portal.modePopup') }}</el-radio>
                  </el-radio-group>
                </el-form-item>
                <template v-if="selectedModule.data.display_mode === 'popup'">
                  <el-form-item :label="t('portal.popupTrigger')">
                    <el-radio-group v-model="selectedModule.data.popup_trigger">
                      <el-radio value="on_load">{{ t('portal.triggerOnLoad') }}</el-radio>
                      <el-radio value="manual">{{ t('portal.triggerManual') }}</el-radio>
                    </el-radio-group>
                  </el-form-item>
                  <el-form-item :label="t('portal.popupDismiss')">
                    <el-radio-group v-model="selectedModule.data.popup_dismiss">
                      <el-radio value="once">{{ t('portal.dismissOnce') }}</el-radio>
                      <el-radio value="always">{{ t('portal.dismissAlways') }}</el-radio>
                    </el-radio-group>
                  </el-form-item>
                  <el-form-item :label="t('portal.popupWidth')">
                    <el-select v-model="selectedModule.data.popup_width" style="width: 100%">
                      <el-option :label="t('portal.widthNarrow')" value="360px" />
                      <el-option :label="t('portal.widthStandard')" value="480px" />
                      <el-option :label="t('portal.widthWide')" value="640px" />
                      <el-option :label="t('portal.widthExtraWide')" value="800px" />
                    </el-select>
                  </el-form-item>
                  <el-form-item :label="t('portal.popupCloseBtn')">
                    <el-input v-model="selectedModule.data.popup_button_text" :placeholder="t('portal.popupCloseBtnPlaceholder')" />
                  </el-form-item>
                  <el-form-item :label="t('portal.popupShowClose')">
                    <el-switch v-model="selectedModule.data.popup_show_close" :active-value="true" :inactive-value="false" />
                  </el-form-item>
                  <el-divider content-position="left">{{ t('portal.popupLinkSection') }}</el-divider>
                  <el-form-item :label="t('portal.heroButtonText')">
                    <el-input v-model="selectedModule.data.popup_link_text" :placeholder="t('portal.popupLinkTextPlaceholder')" />
                  </el-form-item>
                  <el-form-item :label="t('portal.popupLinkUrl')">
                    <el-input v-model="selectedModule.data.popup_link_url" placeholder="https://example.com" />
                  </el-form-item>
                  <el-form-item v-if="selectedModule.data.popup_link_url" :label="t('portal.openInNewWindow')">
                    <el-switch v-model="selectedModule.data.popup_link_external" :active-value="true" :inactive-value="false" />
                  </el-form-item>
                </template>
                <el-alert v-if="selectedModule.data.display_mode === 'popup'" type="info" :closable="false" show-icon style="margin-top: 4px">
                  <span style="font-size: 12px">{{ t('portal.noticePopupHint') }}</span>
                </el-alert>
              </template>

              <!-- Feature Cards Properties -->
              <template v-if="selectedModule.type === 'feature_cards'">
                <div v-for="(card, ci) in selectedModule.data.cards" :key="ci" class="card-edit-row">
                  <el-input v-model="card.icon" :placeholder="t('portal.cardIcon')" style="width: 60px" />
                  <el-input v-model="card.title" :placeholder="t('portal.cardTitle')" />
                  <el-input v-model="card.description" :placeholder="t('portal.cardDesc')" />
                  <el-button text type="danger" :icon="Delete" @click="selectedModule.data.cards.splice(ci, 1)" />
                </div>
                <el-button text :icon="Plus" @click="selectedModule.data.cards.push({ icon: '⭐', title: '', description: '' })">{{ t('portal.addCard') }}</el-button>
              </template>

              <!-- Account Stats Properties -->
              <template v-if="selectedModule.type === 'account_stats'">
                <el-form-item :label="t('portal.layout')">
                  <el-radio-group v-model="selectedModule.style.layout">
                    <el-radio value="grid">{{ t('portal.layoutGrid') }}</el-radio><el-radio value="row">{{ t('portal.layoutRow') }}</el-radio>
                  </el-radio-group>
                </el-form-item>
              </template>

              <!-- Subscription Properties -->
              <template v-if="selectedModule.type === 'subscription'">
                <el-form-item :label="t('portal.heroHeading')"><el-input v-model="selectedModule.data.title" :placeholder="t('portal.subInfoPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.description')"><el-input v-model="selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
                <el-form-item :label="t('portal.heroButtonText')"><el-input v-model="selectedModule.data.button_text" :placeholder="t('portal.copySubLinkPlaceholder')" /></el-form-item>
              </template>

              <!-- Rich Text Properties (editor is in center column) -->
              <template v-if="selectedModule.type === 'rich_text'">
                <el-form-item :label="t('portal.alignment')">
                  <el-radio-group v-model="selectedModule.style.align">
                    <el-radio value="left">{{ t('portal.alignLeft') }}</el-radio><el-radio value="center">{{ t('portal.alignCenter') }}</el-radio>
                  </el-radio-group>
                </el-form-item>
              </template>

              <!-- Custom HTML Properties (editor is in center column) -->
              <template v-if="selectedModule.type === 'custom_html'">
                <el-alert type="info" :closable="false" show-icon style="margin-bottom: 8px">
                  <span style="font-size: 12px">{{ t('portal.codeEditorAreaHint') }}</span>
                </el-alert>
              </template>

              <!-- Subscription Detail Properties -->
              <template v-if="selectedModule.type === 'subscription_detail'">
                <el-form-item :label="t('portal.description')"><el-input v-model="selectedModule.data.description" type="textarea" :rows="2" :placeholder="t('portal.subDetailDescPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.copyBtnText')"><el-input v-model="selectedModule.data.copy_text" :placeholder="t('portal.copyPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.qrBtnText')"><el-input v-model="selectedModule.data.qr_text" :placeholder="t('portal.qrPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.showSubStatus')"><el-switch v-model="selectedModule.data.show_status" /></el-form-item>
                <el-form-item :label="t('portal.showRotateToken')"><el-switch v-model="selectedModule.data.show_rotate" /></el-form-item>
              </template>

              <!-- Security Settings Properties -->
              <template v-if="selectedModule.type === 'security_settings'">
                <el-form-item :label="t('portal.description')"><el-input v-model="selectedModule.data.description" type="textarea" :rows="2" :placeholder="t('portal.securityDescPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.showIpWhitelist')"><el-switch v-model="selectedModule.data.show_ip_whitelist" /></el-form-item>
                <el-form-item :label="t('portal.showChangePassword')"><el-switch v-model="selectedModule.data.show_password" /></el-form-item>
              </template>

              <!-- Log Packages Properties -->
              <template v-if="selectedModule.type === 'log_packages'">
                <el-form-item :label="t('portal.description')"><el-input v-model="selectedModule.data.description" type="textarea" :rows="2" :placeholder="t('portal.logPkgDescPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.btn1h')"><el-input v-model="selectedModule.data.btn_1h" :placeholder="t('portal.btn1hPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.btn6h')"><el-input v-model="selectedModule.data.btn_6h" :placeholder="t('portal.btn6hPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.btn24h')"><el-input v-model="selectedModule.data.btn_24h" :placeholder="t('portal.btn24hPlaceholder')" /></el-form-item>
              </template>

              <!-- Plan List Properties -->
              <template v-if="selectedModule.type === 'plan_list'">
                <el-form-item :label="t('portal.description')"><el-input v-model="selectedModule.data.description" type="textarea" :rows="2" :placeholder="t('portal.optionalPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.showPrice')"><el-switch v-model="selectedModule.data.show_price" /></el-form-item>
                <el-form-item :label="t('portal.showQuota')"><el-switch v-model="selectedModule.data.show_quota" /></el-form-item>
                <el-form-item :label="t('portal.buyBtnText')"><el-input v-model="selectedModule.data.buy_text" :placeholder="t('portal.buyBtnPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.renewBtnText')"><el-input v-model="selectedModule.data.renew_text" :placeholder="t('portal.renewBtnPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.columns')"><el-input-number v-model="selectedModule.data.columns" :min="1" :max="4" /></el-form-item>
              </template>

              <!-- Order Create Properties -->
              <template v-if="selectedModule.type === 'order_create'">
                <el-form-item :label="t('portal.description')"><el-input v-model="selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
              </template>

              <!-- Charge Panel Properties -->
              <template v-if="selectedModule.type === 'charge_panel'">
                <el-form-item :label="t('portal.description')"><el-input v-model="selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
                <el-form-item :label="t('portal.showChargeRecords')"><el-switch v-model="selectedModule.data.show_orders" /></el-form-item>
                <el-form-item :label="t('portal.recordsLimit')"><el-input-number v-model="selectedModule.data.orders_limit" :min="1" :max="20" /></el-form-item>
                <el-form-item :label="t('portal.cardPlaceholder')"><el-input v-model="selectedModule.data.card_placeholder" :placeholder="t('portal.cardPlaceholderText')" /></el-form-item>
                <el-form-item :label="t('portal.redeemBtnText')"><el-input v-model="selectedModule.data.card_btn_text" :placeholder="t('portal.redeemPlaceholder')" /></el-form-item>
              </template>

              <!-- Balance Display Properties -->
              <template v-if="selectedModule.type === 'balance_display'">
                <el-form-item :label="t('portal.showTransactions')"><el-switch v-model="selectedModule.data.show_transactions" /></el-form-item>
                <el-form-item :label="t('portal.txLimit')"><el-input-number v-model="selectedModule.data.tx_limit" :min="1" :max="50" /></el-form-item>
                <el-form-item :label="t('portal.chargeBtnText2')"><el-input v-model="selectedModule.data.charge_btn_text" :placeholder="t('portal.chargeBtnPlaceholder')" /></el-form-item>
              </template>

              <!-- Referral Properties -->
              <template v-if="selectedModule.type === 'referral'">
                <el-form-item :label="t('portal.description')"><el-input v-model="selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
                <el-form-item :label="t('portal.copyBtnText')"><el-input v-model="selectedModule.data.copy_btn_text" :placeholder="t('portal.copyInvitePlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.showInviteRecords')"><el-switch v-model="selectedModule.data.show_records" /></el-form-item>
                <el-form-item :label="t('portal.recordsLimit')"><el-input-number v-model="selectedModule.data.records_limit" :min="1" :max="50" /></el-form-item>
              </template>

              <!-- Ticket List Properties -->
              <template v-if="selectedModule.type === 'ticket_list'">
                <el-form-item :label="t('portal.newTicketBtn')"><el-input v-model="selectedModule.data.new_btn_text" :placeholder="t('portal.newTicketPlaceholder')" /></el-form-item>
              </template>

              <!-- Article List Properties -->
              <template v-if="selectedModule.type === 'article_list'">
                <el-form-item :label="t('portal.showCategoryFilter')"><el-switch v-model="selectedModule.data.show_category_filter" /></el-form-item>
                <el-form-item :label="t('portal.defaultCategory')">
                  <el-select v-model="selectedModule.data.default_category" style="width: 100%">
                    <el-option :label="t('portal.catAll')" value="" /><el-option :label="t('portal.catTutorial')" value="tutorial" /><el-option :label="t('portal.catFaq')" value="faq" /><el-option :label="t('portal.catNotice')" value="notice" />
                  </el-select>
                </el-form-item>
              </template>

              <!-- Shared Accounts Properties -->
              <template v-if="selectedModule.type === 'shared_accounts'">
                <el-form-item :label="t('portal.description')"><el-input v-model="selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
                <el-form-item :label="t('portal.viewCredBtn')"><el-input v-model="selectedModule.data.apply_btn_text" :placeholder="t('portal.viewCredPlaceholder')" /></el-form-item>
                <el-form-item :label="t('portal.repairBtn')"><el-input v-model="selectedModule.data.repair_btn_text" :placeholder="t('portal.repairBtnPlaceholder')" /></el-form-item>
              </template>
            </el-form>
          </div>

          <!-- No Selection -->
          <div v-else class="props-empty">
            <span class="props-empty-icon">⚙️</span>
            <p>{{ t('portal.selectToEdit') }}</p>
          </div>
        </div>
      </div>

      <!-- ===== Lang Editor Mode ===== -->
      <div v-if="editorMode === 'lang'" class="lang-mode">
        <div class="lang-toolbar">
          <el-input v-model="langSearch" :placeholder="t('portal.searchLangKey')" :prefix-icon="Search" clearable size="small" style="width: 320px" />
          <el-button text size="small" :icon="Plus" @click="addLangKey">{{ t('portal.addLangEntry') }}</el-button>
          <el-button text size="small" @click="exportLang">{{ t('portal.exportJson') }}</el-button>
          <el-upload :show-file-list="false" :before-upload="importLang" accept=".json">
            <el-button text size="small">{{ t('portal.importJson') }}</el-button>
          </el-upload>
          <span class="lang-count">{{ t('portal.langEntryCount', { count: filteredLangEntries.length }) }}</span>
        </div>
        <!-- Locale tabs -->
        <div class="lang-locales-bar">
          <div
            v-for="loc in langAvailableLocales"
            :key="loc.code"
            class="lang-locale-tab"
            :class="{ active: langLocale === loc.code }"
            @click="switchLangLocale(loc.code)"
          >
            <span>{{ loc.flag }} {{ loc.label }}</span>
            <el-icon v-if="loc.code === langPack.default" class="locale-default-badge" :title="t('portal.defaultLang')">★</el-icon>
            <el-icon v-if="loc.code !== langPack.default" class="locale-remove-btn" @click.stop="removeLocale(loc.code)"><Close /></el-icon>
          </div>
          <el-button text size="small" :icon="Plus" @click="addLocale">{{ t('portal.addLocale') }}</el-button>
          <el-select v-model="langPack.default" size="small" style="width: 160px; margin-left: auto;" :placeholder="t('portal.defaultLang')">
            <el-option v-for="loc in langAvailableLocales" :key="loc.code" :label="loc.flag + ' ' + loc.label" :value="loc.code" />
          </el-select>
        </div>
        <div class="lang-table-wrap">
          <table class="lang-table">
            <thead>
              <tr>
                <th style="width: 30%">Key</th>
                <th>{{ langAvailableLocales.find(l => l.code === langLocale)?.flag }} {{ langAvailableLocales.find(l => l.code === langLocale)?.label }} {{ t('portal.langContent') }}</th>
                <th style="width: 80px">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="entry in filteredLangEntries" :key="entry.key">
                <td class="lang-key-cell">
                  <span class="lang-key-prefix" :class="'prefix-' + entry.prefix">{{ entry.prefix }}</span>
                  <span class="lang-key-name">{{ entry.key }}</span>
                </td>
                <td class="lang-value-cell">
                  <el-input v-model="langData[entry.key]" size="small" :placeholder="entry.key" />
                </td>
                <td class="lang-action-cell">
                  <el-button text size="small" type="danger" :icon="Delete" @click="deleteLangKey(entry.key)" />
                </td>
              </tr>
            </tbody>
          </table>
          <div v-if="filteredLangEntries.length === 0" class="lang-empty">
            <span style="font-size: 40px">📝</span>
            <p>{{ t('portal.noLangData') }}</p>
            <el-button text size="small" :icon="Plus" @click="addLangKey">{{ t('portal.addFirstLangEntry') }}</el-button>
          </div>
        </div>
      </div>

      <!-- ===== Settings Mode ===== -->
      <div v-if="editorMode === 'settings'" class="settings-mode">
        <el-tabs v-model="settingsTab" type="border-card">
          <el-tab-pane :label="t('portal.basicSettingsTab')" name="basic">
            <el-form :model="form" label-width="140px" style="max-width: 700px; padding: 20px 0">
              <el-form-item :label="t('portal.siteName')"><el-input v-model="form.name" :placeholder="t('portal.siteNamePlaceholder')" /></el-form-item>
              <el-form-item :label="t('portal.siteDescription')"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
              <el-form-item :label="t('portal.openRegistration')"><el-switch v-model="registrationEnabled" /></el-form-item>
              <el-form-item :label="t('portal.portalPath')"><el-input value="/p" disabled /></el-form-item>
              <el-form-item :label="t('portal.portalPort')"><el-input v-model="portalPort" :placeholder="t('portal.portalPortPlaceholder')" /></el-form-item>
            </el-form>
          </el-tab-pane>
          <el-tab-pane :label="t('portal.appearanceTab')" name="appearance">
            <el-form :model="config" label-width="140px" style="max-width: 700px; padding: 20px 0">
              <el-form-item :label="t('portal.siteTitle')"><el-input v-model="config.site_name" placeholder="CoreFusion" /></el-form-item>
              <el-form-item :label="t('portal.logoUrl')"><el-input v-model="config.logo_url" placeholder="https://..." /></el-form-item>
              <el-form-item :label="t('portal.primaryColor')"><div class="color-row"><el-color-picker v-model="config.primary_color" /><el-input v-model="config.primary_color" style="width: 120px" /></div></el-form-item>
              <el-form-item :label="t('portal.backgroundColor')"><div class="color-row"><el-color-picker v-model="config.background_color" /><el-input v-model="config.background_color" style="width: 120px" /></div></el-form-item>
              <el-form-item :label="t('portal.cardBgColor')"><div class="color-row"><el-color-picker v-model="config.card_background_color" /><el-input v-model="config.card_background_color" style="width: 120px" /></div></el-form-item>
              <el-form-item :label="t('portal.textColor')"><div class="color-row"><el-color-picker v-model="config.text_color" /><el-input v-model="config.text_color" style="width: 120px" /></div></el-form-item>
              <el-form-item :label="t('portal.accentColor')"><div class="color-row"><el-color-picker v-model="config.accent_color" /><el-input v-model="config.accent_color" style="width: 120px" /></div></el-form-item>
              <el-form-item :label="t('portal.fontFamily')"><el-input v-model="config.font_family" /></el-form-item>
            </el-form>
          </el-tab-pane>
          <el-tab-pane :label="t('portal.apiRefTab')" name="api">
            <div class="api-ref-container">
              <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px">
                <span style="font-size: 13px">{{ t('portal.apiRefHint') }}</span>
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
                      <span class="api-auth-label">{{ t('portal.authLabel') }}</span>
                      <span class="api-auth-value">{{ api.auth }}</span>
                    </div>
                    <div v-if="api.requestBody" class="api-section-block">
                      <div class="api-block-label">{{ t('portal.requestExample') }}</div>
                      <div class="api-copy-bar">
                        <el-button text size="small" :icon="CopyDocument" @click="copyText(api.requestBody)">{{ t('common.copy') }}</el-button>
                      </div>
                      <pre class="api-code-block"><code>{{ api.requestBody }}</code></pre>
                    </div>
                    <div v-if="api.response" class="api-section-block">
                      <div class="api-block-label">{{ t('portal.responseExample') }}</div>
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
      <div v-if="editorMode === 'code'" class="code-mode">
        <el-alert type="info" :closable="false" show-icon style="margin-bottom: 0; border-radius: 0">
          <span style="font-size: 13px">
            <b>{{ t('portal.codeModeTitle') }}</b> — {{ t('portal.codeModeHint') }}
            <br/>{{ t('portal.htmlVarsLabel') }}：<code v-pre>{{.PortalName}}</code> <code v-pre>{{.LogoURL}}</code> <code v-pre>{{.RegistrationEnabled}}</code>
            <code v-pre>{{registrationEnabledText}}</code> &nbsp;|&nbsp; {{ t('portal.cssVarsLabel') }}：<code v-pre>{{.PrimaryColor}}</code>
            <code v-pre>{{.BackgroundColor}}</code> <code v-pre>{{.FontFamily}}</code> {{ t('portal.etc') }}
          </span>
        </el-alert>
        <div class="code-tabs-bar">
          <el-radio-group v-model="codeTab" size="small">
            <el-radio-button value="html">{{ t('portal.htmlTemplate') }}</el-radio-button>
            <el-radio-button value="css">{{ t('portal.cssStyles') }}</el-radio-button>
            <el-radio-button value="js">{{ t('portal.javascript') }}</el-radio-button>
            <el-radio-button value="advanced">{{ t('portal.resetAndExport') }}</el-radio-button>
          </el-radio-group>
        </div>
        <div v-if="codeTab !== 'advanced'" class="code-editor-area">
          <CodeEditor
            v-if="codeTab === 'html'"
            v-model="form.html_content"
            language="html"
            :minimap="true"
            @change="onCodeChange"
          />
          <CodeEditor
            v-if="codeTab === 'css'"
            v-model="form.css_content"
            language="css"
            :minimap="true"
            @change="onCodeChange"
          />
          <CodeEditor
            v-if="codeTab === 'js'"
            v-model="form.js_content"
            language="javascript"
            :minimap="true"
            @change="onCodeChange"
          />
        </div>
        <div v-else class="code-advanced-pane">
          <el-alert type="warning" :closable="false" show-icon :title="t('portal.resetToDefault')" style="margin-bottom: 16px">
            t('portal.resetWarning')
          </el-alert>
          <el-button type="danger" @click="handleReset">{{ t('portal.resetToDefaultBtn') }}</el-button>
        </div>
      </div>
    </div>

    <!-- Navigation Dialog -->
    <el-dialog v-model="navDialogVisible" :title="t('portal.addNavPage')" width="500px">
      <el-form label-position="top" size="small">
        <el-form-item :label="t('portal.pageName')"><el-input v-model="newNav.label" :placeholder="t('portal.pageNamePlaceholder')" /></el-form-item>
        <el-form-item :label="t('portal.iconOptional')"><el-input v-model="newNav.icon" :placeholder="t('portal.iconPlaceholder2')" /></el-form-item>
        <el-form-item :label="t('portal.externalLink')"><el-input v-model="newNav.href" :placeholder="t('portal.externalLinkPlaceholder2')" /></el-form-item>
      </el-form>
      <div style="font-size: 13px; color: #909399; margin-top: 8px">{{ t('portal.addNavHint') }}</div>
      <template #footer>
        <el-button @click="navDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="confirmAddNav">{{ t('common.create') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, shallowRef, watch, defineComponent, h, compile } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { View, Check, Plus, Delete, ArrowUp, ArrowDown, CopyDocument, Search, Close } from '@element-plus/icons-vue'
import { getActivePortal, updateActivePortal, resetPortal, getPortalCacheStatus, clearPortalCache, rebuildPortalCache, setPortalDevMode } from '../api/portal'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import CodeEditor from '../components/common/CodeEditor.vue'
import { useQueryState } from '../composables/useQueryState'

const { t } = useI18n()

// Configure marked
marked.setOptions({ breaks: true, gfm: true })

const loading = ref(false)
const saving = ref(false)
const editorMode = useQueryState('mode', 'builder')
const settingsTab = useQueryState('stab', 'basic')
const codeTab = useQueryState('ctab', 'html')
const langSearch = ref('')  // search filter for lang editor
const previewDevice = ref('desktop')
const hasUnsavedChanges = ref(false)
const registrationEnabled = ref(false)
const portalPort = ref('')
const navDialogVisible = ref(false)
const richTextEditRef = ref(null)
const mdEditorRef = shallowRef(null)
const noticeMdEditorRef = shallowRef(null)

const form = reactive({ name: '', description: '', html_content: '', css_content: '', js_content: '' })
// Multi-language i18n pack: { default: 'zh-CN', locales: { 'zh-CN': {...}, 'en': {...} } }
const langPack = ref({ default: 'zh-CN', locales: {} })
const langLocale = ref('zh-CN')  // currently selected locale in editor
const langData = ref({})  // current locale's key -> value pairs
const config = reactive({
  site_name: 'CoreFusion', logo_url: '', primary_color: '#667eea', background_color: '#f0f2f5',
  card_background_color: '#ffffff', text_color: '#303133', accent_color: '#67c23a',
  font_family: "system-ui, -apple-system, sans-serif",
})

const navigation = ref([])
const modules = ref([])
const selectedNavId = ref('')
const selectedModuleId = ref('')

// Regions (editable system pages like auth/login)
const regions = ref({})  // { auth: { html, css, js, data } }
const selectedRegionType = ref('')  // 'auth' | ''
const regionViewMode = useQueryState('rview', 'preview')  // 'preview' | 'html' | 'css' | 'js'
const previewFrame = ref(null)

const newNav = reactive({ label: '', icon: '', target: 'overview', href: '', enabled: true, open_external: false })

const activeApiNames = ref([0])

const apiGroups = computed(() => [
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
])

function copyText(text) {
  navigator.clipboard.writeText(text).then(() => ElMessage.success(t('portal.copiedToClipboard'))).catch(() => ElMessage.error(t('portal.copyFailed')))
}

const selectedNav = computed(() => navigation.value.find(n => n.id === selectedNavId.value))
const selectedModule = computed(() => modules.value.find(m => m.id === selectedModuleId.value))

// Separate state: which page we're viewing (controls left/center panels)
// vs what we're editing in the right panel (nav item or module)
const currentPageNavId = ref('')
const currentNav = computed(() => navigation.value.find(n => n.id === currentPageNavId.value))

// Current page target (derived from the page we're viewing, NOT the editing selection)
const currentPageTarget = computed(() => currentNav.value?.target || '')

// Center column view mode: 'preview' | 'editor' | 'css' | 'page-preview' | 'html' | 'js'
const centerViewMode = useQueryState('cview', 'preview')
const freeformTab = useQueryState('ftab', 'html')

// Whether the selected module has a content code editor (markdown, rich_text, etc.)
const hasContentEditor = computed(() => {
  if (!selectedModule.value) return false
  return ['markdown', 'rich_text', 'notice', 'custom_html', 'freeform'].includes(selectedModule.value.type)
})

// Whether the selected module can have custom CSS
const hasCustomCss = computed(() => !!selectedModule.value)

// Ensure module.style.custom_css exists
function ensureCustomCss(mod) {
  if (!mod) return
  if (!mod.style) mod.style = {}
  if (typeof mod.style.custom_css !== 'string') mod.style.custom_css = ''
}

// Ensure notice module has popup-related data fields initialized
function ensureNoticeData(mod) {
  if (!mod || mod.type !== 'notice') return
  if (!mod.data) mod.data = {}
  const d = mod.data
  if (typeof d.display_mode !== 'string') d.display_mode = 'inline'
  if (typeof d.popup_trigger !== 'string') d.popup_trigger = 'on_load'
  if (typeof d.popup_dismiss !== 'string') d.popup_dismiss = 'once'
  if (typeof d.popup_width !== 'string') d.popup_width = '480px'
  if (typeof d.popup_button_text !== 'string') d.popup_button_text = t('portal.api.ex.defaultPopupButton')
  if (typeof d.popup_show_close !== 'boolean') d.popup_show_close = true
  if (typeof d.popup_link_text !== 'string') d.popup_link_text = ''
  if (typeof d.popup_link_url !== 'string') d.popup_link_url = ''
  if (typeof d.popup_link_external !== 'boolean') d.popup_link_external = true
}

// When switching to CSS mode, auto-fill custom_css if empty
function onSwitchToCssMode() {
  if (!selectedModule.value) return
  ensureCustomCss(selectedModule.value)
}

// Watch centerViewMode: when switching to 'css', auto-fill default CSS
watch(centerViewMode, (newVal) => {
  if (newVal === 'css') onSwitchToCssMode()
})

// Auto-switch center view mode when selecting a module
function autoCenterViewMode() {
  if (hasContentEditor.value) centerViewMode.value = 'editor'
  else centerViewMode.value = 'preview'
}

// Computed custom CSS for the currently selected module (for live preview)
const selectedModuleCustomCss = computed(() => {
  if (!selectedModule.value) return ''
  ensureCustomCss(selectedModule.value)
  return selectedModule.value.style.custom_css || ''
})

// Collect all custom CSS from all visible modules for the full preview
const allModuleCustomCss = computed(() => {
  return modules.value
    .filter(m => m.visible && m.style?.custom_css)
    .map(m => {
      const css = m.style.custom_css
      // Scope CSS to the module's wrapper: #preview-mod-{id}
      // Replace bare selectors with scoped ones
      return `\n/* Module: ${m.title || m.id} */\n${css}`
    })
    .join('\n')
})

// Modules belonging to the current page
const currentPageModules = computed(() => {
  if (!currentPageTarget.value) return []
  return modules.value.filter(m => (m.page || 'overview') === currentPageTarget.value)
})

// Region computed properties
const selectedRegion = computed(() => {
  if (!selectedRegionType.value) return null
  return regions.value[selectedRegionType.value] || null
})

const selectedRegionSource = computed(() => {
  if (!selectedRegion.value) return { html: '', css: '', js: '' }
  return {
    html: selectedRegion.value.html || '',
    css: selectedRegion.value.css || '',
    js: selectedRegion.value.js || '',
  }
})

const previewUrl = computed(() => {
  return window.location.origin + '/p?preview=1&screen=auth'
})

// Dynamic component for rendering user-edited HTML/CSS/JS
const dynamicModuleComponent = computed(() => {
  if (!selectedModule.value) return null
  const mod = selectedModule.value
  const html = mod.data?.html || ''
  const css = mod.data?.css || ''
  const js = mod.data?.js || ''
  
  if (!html && !css && !js) return null
  
  try {
    // Create scoped CSS
    const scopedCss = css ? `
      <style>
        #dynamic-mod-${mod.id} {
          ${css}
        }
      </style>
    ` : ''
    
    // Compile HTML template
    const renderFn = html ? compile(html) : () => h('div', 'No template')
    
    // Execute JS to get component logic
    let componentLogic = {}
    if (js) {
      try {
        // Create a safe context with common Vue APIs
        const ctx = {
          ref,
          reactive,
          computed,
          watch,
          onMounted,
          // Add module data
          data: mod.data || {},
          // Add common helpers
          ElMessage
        }
        // Wrap JS in a function that returns the component logic
        const fn = new Function('ctx', `
          with (ctx) {
            ${js}
          }
        `)
        const result = fn(ctx)
        if (result && typeof result === 'object') {
          componentLogic = result
        }
      } catch (jsErr) {
        console.error('JS execution error:', jsErr)
      }
    }
    
    // Create dynamic component
    return defineComponent({
      name: `DynamicModule-${mod.id}`,
      render: renderFn,
      setup() {
        return componentLogic
      }
    })
  } catch (err) {
    console.error('Dynamic component creation error:', err)
    return null
  }
})

// Function to create dynamic component for any module (used in page preview)
function getDynamicComponent(mod) {
  if (!mod) return null
  const html = mod.data?.html || ''
  const css = mod.data?.css || ''
  const js = mod.data?.js || ''
  
  if (!html && !css && !js) return null
  
  try {
    // Compile HTML template
    const renderFn = html ? compile(html) : () => h('div', 'No template')
    
    // Execute JS to get component logic
    let componentLogic = {}
    if (js) {
      try {
        const ctx = {
          ref,
          reactive,
          computed,
          watch,
          onMounted,
          data: mod.data || {},
          ElMessage
        }
        const fn = new Function('ctx', `
          with (ctx) {
            ${js}
          }
        `)
        const result = fn(ctx)
        if (result && typeof result === 'object') {
          componentLogic = result
        }
      } catch (jsErr) {
        console.error('JS execution error:', jsErr)
      }
    }
    
    return defineComponent({
      name: `DynamicModule-${mod.id}`,
      render: renderFn,
      setup() {
        return componentLogic
      }
    })
  } catch (err) {
    console.error('Dynamic component creation error:', err)
    return null
  }
}

// Iframe-based page preview URL for modules (references actual portal page)
const modulePreviewUrl = computed(() => {
  return window.location.origin + '/p?preview=1'
})

// Count modules for a given page target
function currentPageModuleCount(target) {
  return modules.value.filter(m => (m.page || 'overview') === target).length
}

const previewStyles = computed(() => ({
  '--pv-primary': config.primary_color || '#667eea',
  '--pv-bg': config.background_color || '#f0f2f5',
  '--pv-card-bg': config.card_background_color || '#fff',
  '--pv-text': config.text_color || '#303133',
  '--pv-accent': config.accent_color || '#67c23a',
  '--pv-font': config.font_family || 'sans-serif',
}))

const defaultNavigation = [
  { id: 'nav-overview', label: t('portal.seed.nav.overview'), icon: '📊', target: 'overview', href: '', enabled: true, open_external: false },
  { id: 'nav-subscription', label: t('portal.seed.nav.subscription'), icon: '📋', target: 'subscription', href: '', enabled: true, open_external: false },
  { id: 'nav-store', label: t('portal.seed.nav.store'), icon: '💎', target: 'store', href: '', enabled: true, open_external: false },
  { id: 'nav-security', label: t('portal.seed.nav.security'), icon: '🔒', target: 'security', href: '', enabled: true, open_external: false },
  { id: 'nav-support', label: t('portal.seed.nav.support'), icon: '💬', target: 'support', href: '', enabled: true, open_external: false },
  { id: 'nav-logs', label: t('portal.seed.nav.logs'), icon: '📦', target: 'logs', href: '', enabled: true, open_external: false },
]

const defaultModules = [
  {
    id: 'mod-hero', type: 'hero', title: t('portal.seed.mod.hero.title'), page: 'overview', visible: true,
    data: { eyebrow: 'WELCOME BACK', heading: t('portal.seed.mod.hero.heading'), description: t('portal.seed.mod.hero.desc'), button_text: t('portal.seed.mod.hero.btn'), button_target: 'subscription' },
    style: { variant: 'primary', align: 'left' },
  },
  {
    id: 'mod-account', type: 'account_info', title: t('portal.seed.mod.account.title'), page: 'overview', visible: true,
    data: {}, style: {},
  },
  {
    id: 'mod-traffic', type: 'traffic_ring', title: t('portal.seed.mod.traffic.title'), page: 'overview', visible: true,
    data: {}, style: {},
  },
  {
    id: 'md-notice', type: 'notice', title: t('portal.seed.mod.notice.title'), page: 'overview', visible: true,
    data: { title: t('portal.seed.mod.notice.dataTitle'), markdown: t('portal.seed.mod.notice.markdown') },
    style: { variant: 'info' },
  },
  {
    id: 'mod-sub', type: 'subscription_detail', title: t('portal.seed.mod.sub.title'), page: 'subscription', visible: true,
    data: { description: t('portal.seed.mod.sub.desc'), copy_text: t('portal.seed.common.copy'), qr_text: t('portal.seed.common.qr'), show_status: true, show_rotate: true }, style: {},
  },
  {
    id: 'mod-sec', type: 'security_settings', title: t('portal.seed.mod.sec.title'), page: 'security', visible: true,
    data: { description: t('portal.seed.mod.sec.desc'), show_ip_whitelist: true, show_password: true }, style: {},
  },
  {
    id: 'mod-log', type: 'log_packages', title: t('portal.seed.mod.log.title'), page: 'logs', visible: true,
    data: { description: t('portal.seed.mod.log.desc'), btn_1h: t('portal.seed.common.btn1h'), btn_6h: t('portal.seed.common.btn6h'), btn_24h: t('portal.seed.common.btn24h') }, style: {},
  },
  {
    id: 'mod-log-info', type: 'markdown', title: t('portal.seed.mod.logInfo.title'), page: 'logs', visible: true,
    data: { markdown: t('portal.seed.mod.logInfo.markdown') },
    style: { align: 'left' },
  },
  {
    id: 'mod-plan-list', type: 'plan_list', title: t('portal.seed.mod.plan.title'), page: 'store', visible: true,
    data: { show_price: true, show_features: true, show_quota: true, button_text: t('portal.seed.common.buy'), buy_text: t('portal.seed.common.buyNow'), renew_text: t('portal.seed.common.renew'), columns: 2 },
    style: {},
  },
  {
    id: 'mod-balance', type: 'balance_display', title: t('portal.seed.mod.balance.title'), page: 'store', visible: true,
    data: { show_transactions: true, charge_btn_text: t('portal.seed.common.charge'), tx_limit: 10 },
    style: {},
  },
  {
    id: 'mod-charge', type: 'charge_panel', title: t('portal.seed.mod.charge.title'), page: 'store', visible: false,
    data: { description: t('portal.seed.mod.charge.desc'), show_orders: true, orders_limit: 5, card_placeholder: t('portal.seed.mod.charge.cardPlaceholder'), card_btn_text: t('portal.seed.mod.charge.cardBtn') },
    style: {},
  },
  {
    id: 'mod-referral', type: 'referral', title: t('portal.seed.mod.referral.title'), page: 'support', visible: true,
    data: { description: t('portal.seed.mod.referral.desc'), copy_btn_text: t('portal.seed.mod.referral.copyBtn'), show_records: true, records_limit: 10 },
    style: {},
  },
  {
    id: 'mod-tickets', type: 'ticket_list', title: t('portal.seed.mod.tickets.title'), page: 'support', visible: true,
    data: { new_btn_text: t('portal.seed.mod.tickets.newBtn'), show_categories: true },
    style: {},
  },
  {
    id: 'mod-articles', type: 'article_list', title: t('portal.seed.mod.articles.title'), page: 'support', visible: true,
    data: { show_category_filter: true, default_category: '' },
    style: {},
  },
  {
    id: 'mod-shared-accts', type: 'shared_accounts', title: t('portal.seed.mod.shared.title'), page: 'support', visible: true,
    data: { description: t('portal.seed.mod.shared.desc'), apply_btn_text: t('portal.seed.mod.shared.applyBtn'), repair_btn_text: t('portal.seed.mod.shared.repairBtn') },
    style: {},
  },
]

function moduleTypeLabel(type) {
  const map = {
    hero: t('portal.modHero'), markdown: 'Markdown', notice: t('portal.modNotice'), freeform: t('portal.modFreeform'),
    tutorial_cards: t('portal.modTutorialCards'), article_list: t('portal.modArticleList'),
    account_info: t('portal.modAccountInfo'), traffic_ring: t('portal.modTrafficRing'), subscription_detail: t('portal.modSubDetail'),
    client_download: t('portal.modClientDownload'), streaming_status: t('portal.modStreamingStatus'), shared_accounts: t('portal.modSharedAccounts'),
    plan_list: t('portal.modPlanList'), charge_panel: t('portal.modChargePanel'), balance_display: t('portal.modBalanceDisplay'),
    order_list: t('portal.modOrderList'), referral: t('portal.modReferral'),
    node_list: t('portal.modNodeList'), ticket_list: t('portal.modTicketList'), security_settings: t('portal.modSecuritySettings'), log_packages: t('portal.modLogPackages'),
  }
  return map[type] || type
}

function moduleStyle(mod) {
  const s = {}
  if (mod.style?.align === 'center') s.textAlign = 'center'
  return s
}

// Scope CSS to a specific module preview by prefixing selectors with #preview-mod-{id}
function scopeCss(css, modId) {
  if (!css) return ''
  const scope = `#preview-mod-${modId}`
  // Add scope prefix to each selector in comma-separated selector lists
  return css.replace(/([^{}]+)\{/g, (match, selectors) => {
    const scoped = selectors.split(',').map(s => {
      const trimmed = s.trim()
      if (!trimmed) return trimmed
      // Don't scope @media, @keyframes, etc.
      if (trimmed.startsWith('@')) return trimmed
      return `${scope} ${trimmed}`
    }).join(', ')
    return scoped + ' {'
  })
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

function selectNav(id) {
  currentPageNavId.value = id  // switch page view
  selectedNavId.value = id     // show nav properties in right panel
  selectedModuleId.value = '' // clear module editing
  selectedRegionType.value = '' // clear region selection
}
function selectModule(id) {
  selectedModuleId.value = id  // show module properties in right panel
  selectedNavId.value = ''     // clear nav editing, but keep currentPageNavId
  selectedRegionType.value = '' // clear region selection
  const mod = modules.value.find(m => m.id === id)
  if (mod) {
    ensureCustomCss(mod)
    ensureNoticeData(mod)
    // Force reactivity update by creating new object reference
    const idx = modules.value.findIndex(m => m.id === id)
    if (idx !== -1) {
      modules.value[idx] = { ...mod }
    }
  }
  autoCenterViewMode()
}

function selectRegion(type) {
  selectedRegionType.value = type
  selectedNavId.value = ''
  selectedModuleId.value = ''
  currentPageNavId.value = ''
  regionViewMode.value = 'preview'
}

function openNavDialog() {
  Object.assign(newNav, { label: '', icon: '', target: '', href: '', enabled: true, open_external: false })
  navDialogVisible.value = true
}
function confirmAddNav() {
  if (!newNav.label) { ElMessage.error(t('portal.pleaseEnterName')); return }
  // Auto-generate target from label if not set
  const target = newNav.target || newNav.label.trim().toLowerCase().replace(/[^a-z0-9\u4e00-\u9fa5]+/g, '-').replace(/^-+|-+$/g, '') || 'page-' + Date.now()
  const navId = genId('nav')
  navigation.value.push({ ...newNav, id: navId, target })
  navDialogVisible.value = false
  // Auto-select the new page
  selectNav(navId)
  hasUnsavedChanges.value = true
}
function removeNav(idx) {
  ElMessageBox.confirm(t('portal.confirmDeleteNav'), t('common.confirm'), { type: 'warning' }).then(() => {
    const removedId = navigation.value[idx].id
    if (selectedNavId.value === removedId) selectedNavId.value = ''
    if (currentPageNavId.value === removedId) {
      // Switch to next available page
      const next = navigation.value[idx + 1] || navigation.value[idx - 1]
      currentPageNavId.value = next ? next.id : ''
    }
    navigation.value.splice(idx, 1)
    hasUnsavedChanges.value = true
  }).catch(() => {})
}

function addModule(type) {
  const defaults = {
    hero: { data: { eyebrow: '', heading: t('portal.dft.hero.heading'), description: '', button_text: '', button_target: 'subscription' }, style: { variant: 'primary', align: 'left', custom_css: '' } },
    markdown: { data: { markdown: t('portal.dft.markdown') }, style: { align: 'left', custom_css: '' } },
    rich_text: { data: { html: t('portal.dft.richText') }, style: { align: 'left', custom_css: '' } },
    notice: { data: { title: t('portal.dft.notice.title'), markdown: t('portal.dft.notice.markdown'), display_mode: 'inline', popup_trigger: 'on_load', popup_dismiss: 'once', popup_width: '480px', popup_button_text: t('portal.api.ex.defaultPopupButton'), popup_show_close: true, popup_link_text: '', popup_link_url: '', popup_link_external: true }, style: { variant: 'info', custom_css: '' } },
    feature_cards: { data: { cards: [{ icon: '⭐', title: t('portal.dft.featureCards.title'), description: t('portal.dft.featureCards.desc') }] }, style: { custom_css: '' } },
    account_stats: { data: {}, style: { layout: 'grid', custom_css: '' } },
    account_info: { data: {}, style: { custom_css: '' } },
    traffic_ring: { data: {}, style: { custom_css: '' } },
    subscription_detail: { data: { description: '', copy_text: t('portal.seed.common.copy'), qr_text: t('portal.seed.common.qr'), show_status: true, show_rotate: true }, style: { custom_css: '' } },
    security_settings: { data: { description: '', show_ip_whitelist: true, show_password: true }, style: { custom_css: '' } },
    log_packages: { data: { description: '', btn_1h: t('portal.seed.common.btn1h'), btn_6h: t('portal.seed.common.btn6h'), btn_24h: t('portal.seed.common.btn24h') }, style: { custom_css: '' } },
    subscription: { data: { title: t('portal.seed.mod.sub.title'), description: '', button_text: t('portal.dft.subscription.btn') }, style: { custom_css: '' } },
    custom_html: { data: { html: t('portal.dft.customHtml') }, style: { custom_css: '' } },
    freeform: {
      data: {
        html: t('portal.dft.freeform.html'),
        css: '.my-module {\n  padding: 20px;\n  border-radius: 12px;\n  background: var(--accent-light);\n}\n.my-module h3 {\n  color: var(--accent);\n  margin-bottom: 8px;\n}\n.my-module button {\n  padding: 6px 16px;\n  border: none;\n  border-radius: 6px;\n  background: var(--accent);\n  color: #fff;\n  cursor: pointer;\n}',
        js: t('portal.dft.freeform.js')
      },
      style: { custom_css: '', custom_class: '' }
    },
    plan_list: { data: { show_price: true, show_features: true, show_quota: true, button_text: t('portal.seed.common.buy'), buy_text: t('portal.seed.common.buyNow'), renew_text: t('portal.seed.common.renew'), columns: 2 }, style: { custom_css: '' } },
    order_create: { data: { description: t('portal.dft.orderCreate.desc') }, style: { custom_css: '' } },
    charge_panel: { data: { description: t('portal.seed.mod.charge.desc'), show_orders: true, orders_limit: 5, card_placeholder: t('portal.seed.mod.charge.cardPlaceholder'), card_btn_text: t('portal.seed.mod.charge.cardBtn') }, style: { custom_css: '' } },
    balance_display: { data: { show_transactions: true, charge_btn_text: t('portal.seed.common.charge'), tx_limit: 10 }, style: { custom_css: '' } },
    referral: { data: { description: t('portal.seed.mod.referral.desc'), copy_btn_text: t('portal.seed.mod.referral.copyBtn'), show_records: true, records_limit: 10 }, style: { custom_css: '' } },
    ticket_list: { data: { new_btn_text: t('portal.seed.mod.tickets.newBtn'), show_categories: true }, style: { custom_css: '' } },
    article_list: { data: { show_category_filter: true, default_category: '' }, style: { custom_css: '' } },
    shared_accounts: { data: { description: t('portal.seed.mod.shared.desc'), apply_btn_text: t('portal.seed.mod.shared.applyBtn'), repair_btn_text: t('portal.seed.mod.shared.repairBtn') }, style: { custom_css: '' } },
  }
  const page = currentPageTarget.value || 'overview'
  const mod = { id: genId('mod'), type, title: moduleTypeLabel(type), page, visible: true, data: defaults[type].data, style: defaults[type].style }
  modules.value.push(mod)
  selectedModuleId.value = mod.id
  hasUnsavedChanges.value = true
}

function duplicateModuleByIdx(pageIdx) {
  const mod = currentPageModules.value[pageIdx]
  if (!mod) return
  const realIdx = modules.value.indexOf(mod)
  const copy = JSON.parse(JSON.stringify(mod))
  copy.id = genId('mod')
  copy.title = (mod.title || '') + t('portal.copySuffix')
  modules.value.splice(realIdx + 1, 0, copy)
  hasUnsavedChanges.value = true
}

function removeModuleByIdx(pageIdx) {
  const mod = currentPageModules.value[pageIdx]
  if (!mod) return
  ElMessageBox.confirm(t('portal.confirmDeleteModule'), t('common.confirm'), { type: 'warning' }).then(() => {
    const realIdx = modules.value.indexOf(mod)
    if (realIdx >= 0) {
      if (selectedModuleId.value === mod.id) selectedModuleId.value = ''
      modules.value.splice(realIdx, 1)
      hasUnsavedChanges.value = true
    }
  }).catch(() => {})
}

function movePageModule(pageIdx, dir) {
  const mod = currentPageModules.value[pageIdx]
  if (!mod) return
  const realIdx = modules.value.indexOf(mod)
  // Find the next module on the same page
  const samePageIndices = modules.value.map((m, i) => (m.page || 'overview') === mod.page ? i : -1).filter(i => i >= 0)
  const posInPage = samePageIndices.indexOf(realIdx)
  const targetPos = posInPage + dir
  if (targetPos < 0 || targetPos >= samePageIndices.length) return
  const targetRealIdx = samePageIndices[targetPos]
  const tmp = modules.value[realIdx]
  modules.value[realIdx] = modules.value[targetRealIdx]
  modules.value[targetRealIdx] = tmp
  hasUnsavedChanges.value = true
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
  const url = window.prompt(t('portal.enterLinkUrl'))
  if (url) { document.execCommand('createLink', false, url); syncRichText() }
}
function onRichTextInput() { syncRichText() }
function syncRichText() {
  if (richTextEditRef.value && selectedModule.value) {
    selectedModule.value.data.html = richTextEditRef.value.innerHTML
    hasUnsavedChanges.value = true
  }
}

// Monaco editor mount handlers
function onMdEditorMount(editor) {
  mdEditorRef.value = editor
}
function onNoticeMdEditorMount(editor) {
  noticeMdEditorRef.value = editor
}
function onCodeChange() {
  hasUnsavedChanges.value = true
}

// Markdown editor toolbar
function insertMd(type) {
  const editor = mdEditorRef.value
  if (!editor || !selectedModule.value) return
  const model = editor.getModel()
  if (!model) return
  const selection = editor.getSelection()
  const sel = model.getValueInRange(selection)
  let insert = ''
  switch (type) {
    case 'heading': insert = `## ${sel || t('portal.mdInsert.title')}`; break
    case 'bold': insert = `**${sel || t('portal.mdInsert.bold')}**`; break
    case 'italic': insert = `*${sel || t('portal.mdInsert.italic')}*`; break
    case 'strikethrough': insert = `~~${sel || t('portal.mdInsert.strike')}~~`; break
    case 'ul': insert = `- ${sel || t('portal.mdInsert.listItem')}`; break
    case 'ol': insert = `1. ${sel || t('portal.mdInsert.listItem')}`; break
    case 'checklist': insert = `- [ ] ${sel || t('portal.mdInsert.todo')}`; break
    case 'link': insert = `[${sel || t('portal.mdInsert.linkText')}](https://example.com)`; break
    case 'image': insert = `![${sel || t('portal.mdInsert.imageDesc')}](https://...)`; break
    case 'code': insert = `\`${sel || t('portal.mdInsert.code')}\``; break
    case 'codeblock': insert = `\`\`\`\n${sel || t('portal.mdInsert.codeblock')}\n\`\`\``; break
    case 'quote': insert = `> ${sel || t('portal.mdInsert.quote')}`; break
    case 'table': insert = `t('portal.mdInsert.table')`; break
    case 'hr': insert = `---`; break
  }
  editor.executeEdits('toolbar', [{
    range: selection,
    text: insert,
    forceMoveMarkers: true,
  }])
  hasUnsavedChanges.value = true
}

// Notice MD helpers
function insertNoticeMd(type) {
  const editor = noticeMdEditorRef.value
  if (!editor || !selectedModule.value) return
  const model = editor.getModel()
  if (!model) return
  const selection = editor.getSelection()
  const sel = model.getValueInRange(selection)
  let insert = ''
  switch (type) {
    case 'heading': insert = `## ${sel || t('portal.mdInsert.title')}`; break
    case 'bold': insert = `**${sel || t('portal.mdInsert.boldShort')}**`; break
    case 'italic': insert = `*${sel || t('portal.mdInsert.italicShort')}*`; break
    case 'link': insert = `[${sel || t('portal.mdInsert.linkShort')}](https://...)`; break
    case 'code': insert = `\`${sel || t('portal.mdInsert.code')}\``; break
  }
  editor.executeEdits('toolbar', [{
    range: selection,
    text: insert,
    forceMoveMarkers: true,
  }])
  hasUnsavedChanges.value = true
}

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
      const rawMods = Array.isArray(cfg.modules) && cfg.modules.length > 0 ? cfg.modules : JSON.parse(JSON.stringify(defaultModules))
      modules.value = rawMods.map(m => {
        const mod = { ...m, page: m.page || 'overview' }
        ensureCustomCss(mod)
        ensureNoticeData(mod)
        return mod
      })
    } catch {
      navigation.value = JSON.parse(JSON.stringify(defaultNavigation))
      modules.value = JSON.parse(JSON.stringify(defaultModules))
    }
  } else {
    navigation.value = Array.isArray(d.navigation) && d.navigation.length > 0 ? d.navigation : JSON.parse(JSON.stringify(defaultNavigation))
    const rawMods = Array.isArray(d.modules) && d.modules.length > 0 ? d.modules : JSON.parse(JSON.stringify(defaultModules))
    modules.value = rawMods.map(m => {
      const mod = { ...m, page: m.page || 'overview' }
      ensureCustomCss(mod)
      ensureNoticeData(mod)
      return mod
    })
  }
  // Parse lang pack (multi-language)
  if (d.lang && typeof d.lang === 'object') {
    if (d.lang.locales && typeof d.lang.locales === 'object') {
      langPack.value = d.lang
    } else {
      // Legacy flat format — wrap it
      langPack.value = { default: 'zh-CN', locales: { 'zh-CN': d.lang } }
    }
  } else if (d.config) {
    try {
      const cfg = JSON.parse(d.config)
      if (cfg.lang) {
        if (cfg.lang.locales) langPack.value = cfg.lang
        else langPack.value = { default: 'zh-CN', locales: { 'zh-CN': cfg.lang } }
      } else {
        langPack.value = { default: 'zh-CN', locales: {} }
      }
    } catch { langPack.value = { default: 'zh-CN', locales: {} } }
  } else {
    langPack.value = { default: 'zh-CN', locales: {} }
  }
  // Set current locale to default
  langLocale.value = langPack.value.default || 'zh-CN'
  if (!langPack.value.locales[langLocale.value]) {
    langPack.value.locales[langLocale.value] = {}
  }
  langData.value = langPack.value.locales[langLocale.value]

  // Parse regions (editable system pages)
  if (d.regions && typeof d.regions === 'object') {
    regions.value = d.regions
  } else if (d.config) {
    try {
      const cfg = JSON.parse(d.config)
      regions.value = cfg.regions || {}
    } catch { regions.value = {} }
  } else {
    regions.value = {}
  }

  // Auto-select first nav if none selected
  if (navigation.value.length > 0) {
    if (!currentPageNavId.value) currentPageNavId.value = navigation.value[0].id
    if (!selectedNavId.value) selectedNavId.value = navigation.value[0].id
  }
  hasUnsavedChanges.value = false
}

const fetchConfig = async () => {
  loading.value = true
  try {
    const data = await getActivePortal()
    applyData(data)
  } catch { ElMessage.error(t('portal.loadConfigFailed')) }
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
      lang: Object.keys(langPack.value.locales).length > 0 ? langPack.value : undefined,
      registration_enabled: registrationEnabled.value, portal_port: portalPort.value,
      regions: Object.keys(regions.value).length > 0 ? regions.value : undefined,
    }
    const res = await updateActivePortal(data)
    // Re-apply data from response to sync structured fields
    applyData(res)
    // Rebuild render cache so the portal page reflects changes immediately
    try { await rebuildPortalCache() } catch { /* cache rebuild is best-effort */ }
    ElMessage.success(t('portal.saveSuccess'))
    hasUnsavedChanges.value = false
  } catch (e) { ElMessage.error(e.response?.data?.message || t('portal.saveFailed')) }
  finally { saving.value = false }
}

const handlePreview = () => { window.open(window.location.origin + '/p', '_blank') }

// --- Lang editor helpers ---
// Available locales for the editor
const langAvailableLocales = computed(() => {
  return Object.keys(langPack.value.locales).map(code => {
    const meta = langPack.value.locales[code]?._meta || {}
    return { code, label: meta.label || code, flag: meta.flag || '🌐' }
  })
})

function switchLangLocale(code) {
  // Save current locale data back to pack
  langPack.value.locales[langLocale.value] = langData.value
  // Switch to new locale
  langLocale.value = code
  if (!langPack.value.locales[code]) langPack.value.locales[code] = {}
  langData.value = langPack.value.locales[code]
  hasUnsavedChanges.value = true
}

function addLocale() {
  ElMessageBox.prompt(t('portal.enterLangCode'), t('portal.addLocaleTitle'), {
    confirmButtonText: t('common.create'),
    cancelButtonText: t('common.cancel'),
    inputPattern: /^[a-z]{2}(-[A-Z]{2})?$/,
    inputErrorMessage: t('portal.langCodeFormat')
  }).then(({ value }) => {
    if (langPack.value.locales[value]) { ElMessage.warning(t('portal.langAlreadyExists')); return }
    // Copy keys from current locale with empty values
    const newLocale = {}
    for (const key of Object.keys(langData.value)) {
      if (key === '_meta') continue
      newLocale[key] = ''
    }
    newLocale._meta = { language: value, label: value, flag: '🌐' }
    langPack.value.locales[value] = newLocale
    langPack.value = { ...langPack.value }  // trigger reactivity
    ElMessage.success(t('portal.langAdded', { code: value }))
  }).catch(() => {})
}

function removeLocale(code) {
  if (code === langPack.value.default) { ElMessage.warning(t('portal.cannotDeleteDefaultLang')); return }
  ElMessageBox.confirm(t('portal.confirmDeleteLang', { code }), t('common.confirm'), { type: 'warning' }).then(() => {
    delete langPack.value.locales[code]
    langPack.value = { ...langPack.value }
    if (langLocale.value === code) {
      langLocale.value = langPack.value.default || 'zh-CN'
      langData.value = langPack.value.locales[langLocale.value] || {}
    }
    hasUnsavedChanges.value = true
    ElMessage.success(t('portal.deleted'))
  }).catch(() => {})
}

const filteredLangEntries = computed(() => {
  const entries = []
  const search = langSearch.value.toLowerCase().trim()
  for (const key of Object.keys(langData.value)) {
    if (key.startsWith('_')) continue  // skip meta keys
    const val = String(langData.value[key] || '')
    const prefix = key.split('.')[0] || 'other'
    if (!search || key.toLowerCase().includes(search) || val.toLowerCase().includes(search)) {
      entries.push({ key, val, prefix })
    }
  }
  entries.sort((a, b) => a.key.localeCompare(b.key))
  return entries
})

function addLangKey() {
  ElMessageBox.prompt(t('portal.enterLangKey'), t('portal.addLangEntryTitle'), {
    confirmButtonText: t('common.create'),
    cancelButtonText: t('common.cancel'),
    inputPattern: /^[a-z_]+\.[a-z_]+$/,
    inputErrorMessage: t('portal.langKeyFormat')
  }).then(({ value }) => {
    if (langData.value[value] !== undefined) {
      ElMessage.warning(t('portal.langKeyExists'))
      return
    }
    langData.value[value] = ''
    langData.value = { ...langData.value }  // trigger reactivity
    hasUnsavedChanges.value = true
    ElMessage.success(t('portal.langKeyAdded'))
  }).catch(() => {})
}

function deleteLangKey(key) {
  ElMessageBox.confirm(t('portal.confirmDeleteLangKey', { key }), t('common.confirm'), { type: 'warning' }).then(() => {
    delete langData.value[key]
    langData.value = { ...langData.value }
    hasUnsavedChanges.value = true
    ElMessage.success(t('portal.deleted'))
  }).catch(() => {})
}

function exportLang() {
  // Export the full multi-language pack
  langPack.value.locales[langLocale.value] = langData.value
  const json = JSON.stringify(langPack.value, null, 2)
  const blob = new Blob([json], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'portal-lang.json'
  a.click()
  URL.revokeObjectURL(url)
}

function importLang(file) {
  const reader = new FileReader()
  reader.onload = (e) => {
    try {
      const data = JSON.parse(e.target.result)
      if (data && data.locales && typeof data.locales === 'object') {
        // Multi-language format
        langPack.value = data
        langLocale.value = data.default || Object.keys(data.locales)[0] || 'zh-CN'
        langData.value = langPack.value.locales[langLocale.value] || {}
        hasUnsavedChanges.value = true
        ElMessage.success(t('portal.langPackImported'))
      } else if (typeof data === 'object' && data !== null) {
        // Legacy flat format — merge into current locale
        langData.value = { ...langData.value, ...data }
        langPack.value.locales[langLocale.value] = langData.value
        hasUnsavedChanges.value = true
        ElMessage.success(t('portal.langImportedMerged'))
      } else {
        ElMessage.error(t('portal.invalidFileFormat'))
      }
    } catch {
      ElMessage.error(t('portal.jsonParseFailed'))
    }
  }
  reader.readAsText(file)
  return false  // prevent upload
}

const handleReset = async () => {
  try {
    await ElMessageBox.confirm(t('portal.resetConfirmMsg'), t('portal.resetConfirmTitle'), { type: 'warning' })
    const res = await resetPortal()
    applyData(res)
    try { await rebuildPortalCache() } catch { /* best-effort */ }
    ElMessage.success(t('portal.resetSuccess'))
  } catch (e) { if (e !== 'cancel') ElMessage.error(t('portal.resetFailed')) }
}

// --- Render cache management ---
const cacheStatus = ref({ cached: false, dev_mode: false, hit_count: 0, miss_count: 0 })

async function fetchCacheStatus() {
  try {
    const data = await getPortalCacheStatus()
    cacheStatus.value = data
  } catch { /* ignore */ }
}

async function handleCacheCommand(cmd) {
  if (cmd === 'status') {
    await fetchCacheStatus()
    const s = cacheStatus.value
    const msg = t('portal.cacheStatusCached', { status: s.cached ? t('portal.cachedStatus') : t('portal.notCachedStatus') }) + '\n' +
      t('portal.cacheModeLabel') + '：' + (s.dev_mode ? t('portal.devModeNoCache') : t('portal.prodModeCached')) + '\n' +
      t('portal.hitCount') + '：' + s.hit_count + '\n' +
      t('portal.missCount') + '：' + s.miss_count +
      (s.built_at ? '\n' + t('portal.cacheTime') + '：' + s.built_at : '')
    ElMessageBox.alert(msg, t('portal.cacheStatusTitle'), { confirmButtonText: t('common.close') })
  } else if (cmd === 'rebuild') {
    try {
      await rebuildPortalCache()
      await fetchCacheStatus()
      ElMessage.success(t('portal.cacheRebuilt'))
    } catch { ElMessage.error(t('portal.rebuildCacheFailed')) }
  } else if (cmd === 'clear') {
    try {
      await clearPortalCache()
      await fetchCacheStatus()
      ElMessage.success(t('portal.cacheCleared'))
    } catch { ElMessage.error(t('portal.clearCacheFailed')) }
  } else if (cmd === 'dev') {
    const newMode = !cacheStatus.value.dev_mode
    try {
      await setPortalDevMode(newMode)
      await fetchCacheStatus()
      ElMessage.success(newMode ? t('portal.switchedToDevMode') : t('portal.switchedToProdMode'))
    } catch { ElMessage.error(t('portal.switchModeFailed')) }
  }
}

onMounted(() => { fetchConfig(); fetchCacheStatus() })
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

.editor-body { flex: 1; overflow: hidden; position: relative; min-height: 0; }
.editor-body > div { height: 100%; min-height: 0; }

/* ===== Builder Layout - Clean Card Style ===== */
.builder-layout { display: flex; height: 100%; min-height: 0; background: #f0f2f5; }

/* Left Column - Clean Navigation */
.col-structure {
width: 320px;
background: #fff;
overflow-y: auto;
display: flex;
flex-direction: column;
border-right: 1px solid #e8e8e8;
flex-shrink: 0;
padding: 10px;
gap: 10px;
min-height: 0;
}

.structure-section { background: #fff; }
.structure-section:last-child { flex: 1; display: flex; flex-direction: column; }

.section-header { 
  display: flex; 
  justify-content: space-between; 
  align-items: center; 
  margin-bottom: 8px;
  padding: 0 4px;
}
.section-title { font-size: 11px; font-weight: 600; color: #909399; text-transform: uppercase; letter-spacing: 0.5px; }

/* Big Card Style Navigation Items */
.nav-list { display: flex; flex-direction: column; gap: 4px; max-height: 180px; overflow-y: auto; overflow-x: hidden; }
.nav-item-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 6px;
  cursor: pointer;
  background: #f8f9fa;
  border: 1px solid transparent;
  transition: all 0.2s ease;
}
.nav-item-row:hover { 
  background: #fff; 
  border-color: #d9d9d9;
  box-shadow: 0 2px 8px rgba(0,0,0,0.06);
}
.nav-item-row.selected { 
  background: #e6f7ff; 
  border-color: #1890ff;
  box-shadow: 0 2px 8px rgba(24,144,255,0.15);
}
.nav-item-row.disabled { opacity: 0.5; }

.nav-icon-emoji { font-size: 12px; flex-shrink: 0; }
.nav-item-info { display: flex; align-items: center; gap: 4px; flex: 1; overflow: hidden; min-width: 0; }
.nav-label-text { 
  font-size: 11px; 
  color: #333;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}
.nav-item-info .el-tag { flex-shrink: 0; font-size: 10px; padding: 0 4px; }

.nav-item-actions {
display: flex;
align-items: center;
gap: 2px;
flex-shrink: 0;
}
.nav-item-actions .el-button { padding: 4px; }

/* Module List */
.structure-modules { flex: 1; display: flex; flex-direction: column; overflow: hidden; margin-top: 8px; min-height: 200px; }
.module-list { 
  flex: 1; 
  overflow-y: auto; 
  display: flex; 
  flex-direction: column; 
  gap: 6px;
}
.module-item-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  border-radius: 6px;
  cursor: pointer;
  background: #fafafa;
  border: 1px solid #f0f0f0;
  transition: all 0.2s;
}
.module-item-row:hover { background: #f0f5ff; border-color: #d6e4ff; }
.module-item-row.selected { background: #e6f7ff; border-color: #1890ff; }
.module-item-row.disabled { opacity: 0.5; }

.module-item-info { display: flex; align-items: center; gap: 6px; flex: 1; overflow: hidden; min-width: 0; }
.module-type-badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 500;
  flex-shrink: 0;
}
.module-title-text { 
  font-size: 11px; 
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  min-width: 0;
}
.module-item-actions { 
  display: flex; 
  align-items: center; 
  gap: 1px;
  flex-shrink: 0;
}
.module-item-actions .el-button { padding: 4px; }

/* Center Column - Editor with Toolbar */
.col-canvas {
flex: 1;
background: #f0f2f5;
display: flex;
flex-direction: column;
overflow: hidden;
min-height: 0;
  min-width: 0;
}

/* Toolbar Style */
.canvas-toolbar { 
  display: flex; 
  align-items: center; 
  justify-content: space-between;
  gap: 16px; 
  padding: 12px 20px; 
  background: #fff; 
  border-bottom: 1px solid #e8e8e8; 
  flex-shrink: 0;
}
.canvas-toolbar-left, .canvas-toolbar-right { display: flex; align-items: center; gap: 12px; }

/* Editor Content Area */
.canvas-code-editor { 
  flex: 1; 
  display: flex; 
  flex-direction: column; 
  overflow: hidden; 
  padding: 20px; 
  background: #f0f2f5;
}
.canvas-scroll { 
  flex: 1; 
  overflow: auto; 
  display: flex;
  justify-content: center;
  padding: 20px;
}

/* Right Column - Properties */
.col-properties {
width: 280px;
background: #fff;
overflow-y: auto;
border-left: 1px solid #e8e8e8;
flex-shrink: 0;
padding: 16px;
}

/* Badge Colors */
.badge-hero { background: #f6ffed; color: #52c41a; }
.badge-markdown { background: #e6f7ff; color: #1890ff; }
.badge-rich_text { background: #f0f5ff; color: #2f54eb; }
.badge-notice { background: #fff7e6; color: #fa8c16; }
.badge-feature_cards { background: #f9f0ff; color: #722ed1; }
.badge-account_stats { background: #e6fffb; color: #13c2c2; }
.badge-subscription { background: #fff0f6; color: #eb2f96; }
.badge-custom_html { background: #fff2e8; color: #fa541c; }
.badge-account_info { background: #f6ffed; color: #52c41a; }
.badge-traffic_ring { background: #e6f7ff; color: #1890ff; }
.badge-subscription_detail { background: #fff0f6; color: #eb2f96; }
.badge-security_settings { background: #fff7e6; color: #fa8c16; }
.badge-log_packages { background: #f9f0ff; color: #722ed1; }

.empty-hint { 
  text-align: center; 
  padding: 24px 16px; 
  color: #999; 
  font-size: 13px; 
  cursor: pointer; 
  border: 1px dashed #d9d9d9; 
  border-radius: 8px;
  background: #fafafa;
}
.empty-hint:hover { 
  color: #1890ff; 
  border-color: #1890ff;
  background: #e6f7ff;
}

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
.badge-account_info { background: #e8f5e9; color: #4caf50; }
.badge-traffic_ring { background: #e3f2fd; color: #1976d2; }
.badge-subscription_detail { background: #fce4ec; color: #e91e63; }
.badge-security_settings { background: #fff3e0; color: #ff9800; }
.badge-log_packages { background: #f3e5f5; color: #9c27b0; }

/* ===== Center Code Editor ===== */
.canvas-code-editor { flex: 1; display: flex; flex-direction: column; overflow: hidden; padding: 12px; background: #fff; }
.canvas-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 8px 16px; background: #fff; border-bottom: 1px solid #e4e7ed; flex-shrink: 0; }
.canvas-scroll { flex: 1; overflow: auto; }
.canvas-toolbar-left { display: flex; align-items: center; gap: 8px; }
.canvas-toolbar-right { display: flex; align-items: center; gap: 8px; }

/* ===== CSS Editor Mode ===== */
.canvas-css-editor { flex: 1; display: flex; flex-direction: column; overflow: hidden; padding: 12px; }
.css-editor-info { flex-shrink: 0; }
.css-editor-split { flex: 1; display: flex; min-height: 0; gap: 8px; overflow: hidden; }
.css-editor-pane { flex: 1; min-width: 0; overflow: hidden; }
.css-preview-pane { flex: 1; overflow-y: auto; background: #f5f7fa; border-radius: 6px; }
.css-preview-pane.device-tablet .portal-preview { max-width: 768px; margin: 0 auto; }
.css-preview-pane.device-mobile .portal-preview { max-width: 375px; margin: 0 auto; }
.module-wrap-preview { padding: 16px; }

.md-editor-full, .rte-editor-wrap, .html-editor-wrap, .notice-editor-wrap { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.md-split-full { flex: 1; min-height: 0; }

.rte-split { flex: 1; display: flex; min-height: 0; overflow: hidden; }
.rte-content-full { flex: 1; overflow-y: auto; }
.rte-preview { flex: 1; overflow-y: auto; border-left: 1px solid #e4e7ed; background: #fff; }

.html-split { flex: 1; display: flex; min-height: 0; overflow: hidden; }
.html-editor-pane {
  flex: 1; min-width: 0; border: 1px solid #dcdfe6; border-radius: 6px 0 0 6px; overflow: hidden;
}
.html-preview {
  flex: 1; padding: 12px; border: 1px solid #dcdfe6; border-left: none; border-radius: 0 6px 6px 0;
  overflow-y: auto; background: #fff;
}

/* Device width for preview panes in code editor */
.md-preview-pane.device-desktop { max-width: 100%; }
.md-preview-pane.device-tablet { max-width: 768px; margin: 0 auto; }
.md-preview-pane.device-mobile { max-width: 375px; margin: 0 auto; }
.rte-preview.device-desktop { max-width: 100%; }
.rte-preview.device-tablet { max-width: 768px; margin: 0 auto; }
.rte-preview.device-mobile { max-width: 375px; margin: 0 auto; }
.html-preview.device-desktop { max-width: 100%; }
.html-preview.device-tablet { max-width: 768px; margin: 0 auto; }
.html-preview.device-mobile { max-width: 375px; margin: 0 auto; }

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

.preview-page-title {
  display: flex; align-items: center; gap: 8px;
  padding: 12px 16px; background: var(--pv-card-bg);
  border-bottom: 2px solid var(--pv-primary);
  flex-shrink: 0;
}
.page-title-icon { font-size: 20px; }
.page-title-text { font-size: 18px; font-weight: 700; color: var(--pv-text); }

/* section modules can grow to fill */
.structure-modules .module-list { flex: 1; overflow-y: auto; }

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
.preview-popup-badge { display: inline-block; font-size: 11px; padding: 2px 8px; border-radius: 4px; background: #e8f4fd; color: #409eff; margin-bottom: 8px; font-weight: 600; }
.preview-notice-link { font-size: 11px; color: #409eff; margin-top: 6px; word-break: break-all; }
.notice-popup-hint-preview { font-size: 11px; color: #909399; margin-top: 6px; padding: 4px 8px; background: #f5f7fa; border-radius: 4px; }
.preview-popup-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; padding-top: 8px; border-top: 1px solid rgba(0,0,0,0.06); }
.preview-popup-btn-link { font-size: 12px; color: #409eff; padding: 4px 12px; border: 1px solid #409eff; border-radius: 4px; }
.preview-popup-btn-dismiss { font-size: 12px; color: #fff; padding: 4px 12px; background: #409eff; border-radius: 4px; }

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
.freeform-badge { display: inline-block; padding: 1px 8px; border-radius: 4px; background: #e8f5e9; color: #2e7d32; font-size: 11px; font-weight: 600; margin-bottom: 8px; }

.freeform-editor-wrap { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.freeform-tabs { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.freeform-tabs :deep(.el-tabs__content) { flex: 1; overflow: hidden; }
.freeform-tabs :deep(.el-tab-pane) { height: 100%; display: flex; flex-direction: column; }
.freeform-editor-pane { flex: 1; overflow: hidden; min-height: 300px; }
.freeform-api-ref { margin-top: 8px; }
.freeform-api-ref pre { font-size: 12px; line-height: 1.6; color: #606266; white-space: pre-wrap; word-break: break-all; margin: 0; }
.freeform-api-ref .el-collapse { border: none; }
.freeform-api-ref .el-collapse-item__header { font-size: 12px; height: 32px; line-height: 32px; }

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
.md-editor-pane {
  width: 50%; min-width: 0; overflow: hidden;
}
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

/* ===== Settings Mode ===== */
.settings-mode { padding: 16px; overflow-y: auto; height: 100%; }

/* ===== Template Code Mode (Monaco) ===== */
.code-mode { display: flex; flex-direction: column; height: 100%; overflow: hidden; }
.code-tabs-bar {
  display: flex; align-items: center; gap: 8px; padding: 8px 16px;
  background: #fff; border-bottom: 1px solid #e4e7ed; flex-shrink: 0;
}
.code-editor-area {
  flex: 1; min-height: 0; padding: 8px;
}
.code-editor-area > * {
  height: 100%;
}
.code-advanced-pane { padding: 20px; overflow-y: auto; }
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
.col-structure { width: 200px; }
.col-properties { width: 260px; }
}
@media (max-width: 1100px) {
  .builder-layout { flex-direction: column; }
  .col-structure, .col-properties { width: 100%; max-height: 250px; }
  .col-canvas { flex: 1; min-height: 300px; }
  .md-split { min-height: 180px; }
}

/* ===== Lang Editor Mode ===== */
.lang-mode { height: 100%; width: 100%; display: flex; flex-direction: column; overflow: hidden; position: relative; }
.lang-toolbar {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 16px; background: #fff; border-bottom: 1px solid #e4e7ed;
}
.lang-count { margin-left: auto; font-size: 12px; color: #909399; }
.lang-locales-bar {
  display: flex; align-items: center; gap: 4px; flex-wrap: nowrap;
  padding: 6px 16px; background: #fafbfc; border-bottom: 1px solid #e4e7ed;
  overflow-x: auto; overflow-y: hidden;
  scrollbar-width: thin;
}
.lang-locales-bar::-webkit-scrollbar { height: 6px; }
.lang-locales-bar::-webkit-scrollbar-thumb { background: #c0c4cc; border-radius: 3px; }
.lang-locales-bar::-webkit-scrollbar-track { background: transparent; }
.lang-locale-tab {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 4px 12px; border-radius: 6px; cursor: pointer;
  font-size: 13px; color: #606266; border: 1px solid transparent;
  transition: all 0.15s;
}
.lang-locale-tab:hover { background: #ecf5ff; color: #409eff; }
.lang-locale-tab.active {
  background: #409eff; color: #fff; border-color: #409eff;
}
.locale-default-badge { color: #f7ba2a; font-size: 12px; }
.lang-locale-tab.active .locale-default-badge { color: #ffd700; }
.locale-remove-btn { color: #c0c4cc; font-size: 12px; margin-left: 2px; }
.locale-remove-btn:hover { color: #f56c6c; }
.lang-table-wrap { flex: 1; overflow: auto; padding: 0 16px 16px; min-height: 0; }
.lang-table {
  width: 100%; border-collapse: collapse; font-size: 13px; margin-top: 8px; table-layout: fixed;
}
.lang-table th {
  text-align: left; padding: 8px 12px; background: #f5f7fa;
  border-bottom: 2px solid #e4e7ed; font-weight: 600; color: #606266;
  position: sticky; top: 0; z-index: 1;
}
.lang-table td {
  padding: 6px 12px; border-bottom: 1px solid #ebeef5; vertical-align: middle;
}
.lang-key-cell { display: flex; align-items: center; gap: 6px; overflow: hidden; }
.lang-key-prefix {
  font-size: 10px; font-weight: 700; padding: 2px 6px; border-radius: 10px;
  text-transform: uppercase; letter-spacing: 0.3px; flex-shrink: 0;
}
.lang-key-name { font-family: 'SF Mono', Monaco, monospace; font-size: 12px; color: #606266; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; min-width: 0; }
.prefix-nav { background: #e6f7ff; color: #1890ff; }
.prefix-common { background: #f6ffed; color: #52c41a; }
.prefix-auth { background: #fff7e6; color: #fa8c16; }
.prefix-account { background: #f9f0ff; color: #722ed1; }
.prefix-traffic { background: #e6fffb; color: #13c2c2; }
.prefix-sub { background: #fff0f6; color: #eb2f96; }
.prefix-plan { background: #fffbe6; color: #faad14; }
.prefix-balance { background: #f0f5ff; color: #2f54eb; }
.prefix-order { background: #fff2e8; color: #fa541c; }
.prefix-node { background: #e8f4f8; color: #1890ff; }
.prefix-tutorial { background: #f0f9eb; color: #67c23a; }
.prefix-client { background: #ecf5ff; color: #409eff; }
.prefix-article { background: #fef0f0; color: #f56c6c; }
.prefix-announcement { background: #fdf6ec; color: #e6a23c; }
.prefix-referral { background: #f0f9eb; color: #67c23a; }
.prefix-ticket { background: #e8f4f8; color: #1890ff; }
.prefix-security { background: #fef0f0; color: #f56c6c; }
.prefix-log { background: #f4f4f5; color: #909399; }
.prefix-charge { background: #fff7e6; color: #fa8c16; }
.prefix-shared { background: #f9f0ff; color: #722ed1; }
.prefix-streaming { background: #e6f7ff; color: #1890ff; }
.prefix-theme { background: #f4f4f5; color: #909399; }
.prefix-error { background: #fef0f0; color: #f56c6c; }
.prefix-toast { background: #f0f9eb; color: #67c23a; }
.prefix-hero { background: #fff0f6; color: #eb2f96; }
.lang-key-name { font-family: 'SF Mono', Monaco, monospace; font-size: 12px; color: #606266; }
.lang-value-cell { min-width: 200px; }
.lang-action-cell { text-align: center; }
.lang-empty {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  padding: 60px 20px; color: #909399;
}
.lang-empty p { margin: 8px 0; font-size: 14px; }

/* ===== Region Editor (iframe preview) ===== */
.preview-iframe {
  width: 100%;
  height: 100%;
  border: none;
  background: #f5f7fa;
}
.canvas-scroll:has(.preview-iframe) {
  display: flex;
  flex-direction: column;
}
</style>
