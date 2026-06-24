<template>
    <el-dialog
        :title="fileInfo ? fileInfo.name : ''"
        :visible.sync="visible"
        :before-close="handleClose"
        fullscreen
        class="file-editor-dialog">
        <div class="editor-toolbar">
            <div class="editor-info">
                <span v-if="fileInfo" class="file-meta">
                    <i class="el-icon-document"></i>
                    {{ fileInfo.name }}
                    <el-tag size="mini" type="info">{{ fileInfo.size }}</el-tag>
                    <el-tag size="mini" type="success" v-if="fileInfo.language">{{ fileInfo.language }}</el-tag>
                    <span class="modified-time">{{ fileInfo.modTime }}</span>
                </span>
            </div>
            <div class="editor-actions">
                <el-button-group>
                    <el-button size="small" icon="el-icon-view" @click="togglePreview" :type="showPreview ? 'primary' : ''">{{ $t('preview') }}</el-button>
                    <el-button size="small" icon="el-icon-download" @click="downloadFile">{{ $t('download') }}</el-button>
                    <el-button size="small" icon="el-icon-check" type="success" @click="saveContent" :loading="saving">{{ $t('save') }}</el-button>
                </el-button-group>
            </div>
        </div>
        <div class="editor-container" :class="{ 'with-preview': showPreview }">
            <div class="editor-main">
                <textarea
                    ref="editor"
                    v-model="content"
                    class="code-editor"
                    :style="{ fontFamily: 'Monaco, Menlo, Consolas, monospace', fontSize: '14px' }"
                    @input="onContentChange"
                    spellcheck="false"
                    :placeholder="$t('editorPlaceholder')"></textarea>
                <div class="editor-status">
                    <span>{{ $t('lines') }}: {{ lineCount }}</span>
                    <span>{{ $t('characters') }}: {{ content.length }}</span>
                    <span v-if="hasUnsavedChanges" class="unsaved-indicator">{{ $t('unsaved') }}</span>
                </div>
            </div>
            <div v-if="showPreview" class="editor-preview">
                <div class="preview-header">
                    <span>{{ $t('preview') }}</span>
                    <el-button size="mini" icon="el-icon-close" @click="showPreview = false"></el-button>
                </div>
                <div class="preview-content" v-html="renderedContent"></div>
            </div>
        </div>
    </el-dialog>
</template>

<script>
import { readFile, saveFile, getFileInfo } from '@/api/file'

export default {
    name: 'FileEditor',
    props: {
        value: {
            type: Boolean,
            default: false
        },
        filePath: {
            type: String,
            default: ''
        }
    },
    data() {
        return {
            content: '',
            originalContent: '',
            fileInfo: null,
            saving: false,
            showPreview: false,
            hasUnsavedChanges: false
        }
    },
    computed: {
        visible: {
            get() {
                return this.value
            },
            set(val) {
                this.$emit('input', val)
            }
        },
        lineCount() {
            return this.content ? this.content.split('\n').length : 0
        },
        renderedContent() {
            if (!this.content) return ''
            if (this.isMarkdown) {
                return this.renderMarkdown(this.content)
            }
            return this.escapeHtml(this.content)
        },
        isMarkdown() {
            return this.fileInfo && this.fileInfo.language === 'markdown'
        }
    },
    watch: {
        filePath(newVal) {
            if (newVal) {
                this.loadFile(newVal)
            }
        },
        value(newVal) {
            if (newVal && this.filePath) {
                this.loadFile(this.filePath)
            }
        }
    },
    methods: {
        async loadFile(path) {
            try {
                const sshInfo = this.$store.getters.sshReq
                const [fileRes, infoRes] = await Promise.all([
                    readFile(path, sshInfo),
                    getFileInfo(path, sshInfo)
                ])
                if (fileRes.Msg === 'success') {
                    this.content = fileRes.Data.content
                    this.originalContent = fileRes.Data.content
                    this.fileInfo = fileRes.Data.info
                    this.hasUnsavedChanges = false
                    this.showPreview = this.isMarkdown
                } else {
                    this.$message.error(fileRes.Msg)
                }
                if (infoRes.Msg === 'success') {
                    this.fileInfo = infoRes.Data
                }
            } catch (e) {
                this.$message.error(this.$t('loadFileError'))
            }
        },
        async saveContent() {
            if (!this.hasUnsavedChanges) {
                this.$message.info(this.$t('noChanges'))
                return
            }
            this.saving = true
            try {
                const sshInfo = this.$store.getters.sshReq
                const result = await saveFile(this.filePath, this.content, sshInfo)
                if (result.Msg === 'success') {
                    this.originalContent = this.content
                    this.hasUnsavedChanges = false
                    this.$message.success(this.$t('saveSuccess'))
                    this.$emit('saved')
                } else {
                    this.$message.error(result.Msg)
                }
            } catch (e) {
                this.$message.error(this.$t('saveError'))
            } finally {
                this.saving = false
            }
        },
        onContentChange() {
            this.hasUnsavedChanges = this.content !== this.originalContent
        },
        togglePreview() {
            this.showPreview = !this.showPreview
        },
        downloadFile() {
            const prefix = process.env.NODE_ENV === 'production' ? `${location.origin}` : 'api'
            const downloadUrl = `${prefix}/file/download?path=${this.filePath}&sshInfo=${this.$store.getters.sshReq}`
            window.open(downloadUrl)
        },
        handleClose(done) {
            if (this.hasUnsavedChanges) {
                this.$confirm(this.$t('unsavedChangesConfirm'), this.$t('warning'), {
                    confirmButtonText: this.$t('saveAndClose'),
                    cancelButtonText: this.$t('discardChanges'),
                    type: 'warning'
                }).then(() => {
                    this.saveContent().then(() => {
                        done()
                        this.$emit('close')
                    })
                }).catch(() => {
                    done()
                    this.$emit('close')
                })
            } else {
                done()
                this.$emit('close')
            }
        },
        renderMarkdown(text) {
            let html = text
                .replace(/^### (.*$)/gim, '<h3>$1</h3>')
                .replace(/^## (.*$)/gim, '<h2>$1</h2>')
                .replace(/^# (.*$)/gim, '<h1>$1</h1>')
                .replace(/\*\*(.*?)\*\*/gim, '<strong>$1</strong>')
                .replace(/\*(.*?)\*/gim, '<em>$1</em>')
                .replace(/`(.*?)`/gim, '<code>$1</code>')
                .replace(/\n/gim, '<br>')
            return html
        },
        escapeHtml(text) {
            const div = document.createElement('div')
            div.textContent = text
            return div.innerHTML
        }
    }
}
</script>

<style lang="scss">
.file-editor-dialog {
    .el-dialog {
        display: flex;
        flex-direction: column;
        margin: 0 !important;
    }
    .el-dialog__body {
        flex: 1;
        padding: 0;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }
}
.editor-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 15px;
    background: var(--card-bg);
    border-bottom: 1px solid var(--input-border);
}
.editor-info {
    .file-meta {
        display: flex;
        align-items: center;
        gap: 10px;
        color: var(--text-color);
    }
    .modified-time {
        color: #999;
        font-size: 12px;
    }
}
.editor-container {
    flex: 1;
    display: flex;
    overflow: hidden;
}
.editor-main {
    flex: 1;
    display: flex;
    flex-direction: column;
    position: relative;
}
.code-editor {
    flex: 1;
    width: 100%;
    padding: 15px;
    border: none;
    outline: none;
    resize: none;
    background: #1e1e1e;
    color: #d4d4d4;
    font-family: Monaco, Menlo, Consolas, monospace;
    font-size: 14px;
    line-height: 1.5;
    tab-size: 4;
    white-space: pre;
    overflow: auto;
}
.editor-status {
    display: flex;
    gap: 15px;
    padding: 5px 15px;
    background: #007acc;
    color: white;
    font-size: 12px;
    .unsaved-indicator {
        color: #ffcc00;
    }
}
.editor-preview {
    width: 40%;
    border-left: 1px solid var(--input-border);
    display: flex;
    flex-direction: column;
    background: var(--input-bg);
}
.preview-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 15px;
    background: var(--card-bg);
    border-bottom: 1px solid var(--input-border);
    font-weight: bold;
}
.preview-content {
    flex: 1;
    padding: 15px;
    overflow: auto;
    line-height: 1.6;
    h1, h2, h3 {
        margin: 15px 0 10px;
        color: var(--text-color);
    }
    code {
        background: #f4f4f4;
        padding: 2px 6px;
        border-radius: 3px;
        font-family: Monaco, Menlo, Consolas, monospace;
    }
    strong {
        font-weight: bold;
    }
    em {
        font-style: italic;
    }
}
</style>
