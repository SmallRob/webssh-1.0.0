<template>
    <div class="file-list-wrapper" @dragenter.prevent="onDragEnter" @dragover.prevent @dragleave.prevent="onDragLeave" @drop.prevent="onDrop">
        <!-- 标题行：统计 + 隐藏文件开关 + 排序 -->
        <div class="sftp-title">
            <span>{{ $t('fileManagement') }}</span>
            <span class="entry-count" v-if="!loading">{{ visibleFiles.length }} {{ $t('items') }}</span>
            <span class="title-actions">
                <i class="el-icon-view title-action" :class="{ 'is-active': !showHidden }" :title="$t('showHidden')" @click="showHidden = !showHidden"></i>
                <el-dropdown trigger="click" @command="handleSortCommand">
                    <i class="el-icon-sort title-action" :title="$t('sortBy')"></i>
                    <el-dropdown-menu slot="dropdown">
                        <el-dropdown-item v-for="opt in sortOptions" :key="opt.value" :command="opt.value">
                            <i :class="opt.value === sortValue ? 'el-icon-check' : 'el-icon-blank'"></i> {{ opt.label }}
                        </el-dropdown-item>
                    </el-dropdown-menu>
                </el-dropdown>
            </span>
        </div>

        <!-- 路径行：面包屑（可点击跳转）+ 复制路径；点击进入编辑模式 -->
        <div class="path-row">
            <template v-if="!pathEditing">
                <div class="breadcrumb" :title="$t('clickToEditPath')" @click="startPathEdit">
                    <span class="crumb-inner">
                        <span class="crumb" v-for="seg in crumbs" :key="seg.path" @click.stop="navigateTo(seg.path)">{{ seg.name }}</span>
                    </span>
                </div>
                <i class="el-icon-document-copy path-action" :title="$t('copyPath')" @click="copyPath(currentPath)"></i>
            </template>
            <template v-else>
                <el-input ref="pathInput" v-model="pathDraft" size="mini" @keyup.enter.native="commitPathEdit" @blur="commitPathEdit" :placeholder="$t('currentPath')"></el-input>
            </template>
        </div>

        <!-- 工具行：常规模式 -->
        <div class="toolbar" v-if="selectedNames.length === 0">
            <el-button-group>
                <el-button type="primary" size="mini" icon="el-icon-arrow-up" @click="upDirectory()" :title="$t('upDirectory')"></el-button>
                <el-button type="primary" size="mini" icon="el-icon-s-home" @click="goToHome()" :title="$t('home')"></el-button>
                <el-button type="primary" size="mini" icon="el-icon-refresh" @click="getFileList()" :title="$t('refresh')"></el-button>
            </el-button-group>
            <el-button type="primary" size="mini" icon="el-icon-search" @click="showSearchPanel = !showSearchPanel" :class="{ 'is-plain-active': showSearchPanel }" :title="$t('search')"></el-button>
            <el-dropdown @command="handleUploadCommand" size="mini" trigger="click">
                <el-button type="primary" size="mini" icon="el-icon-upload" :title="$t('uploadFile')"></el-button>
                <el-dropdown-menu slot="dropdown">
                    <el-dropdown-item command="file"><i class="el-icon-document"></i> {{ $t('uploadFile') }}</el-dropdown-item>
                    <el-dropdown-item command="folder"><i class="el-icon-folder-opened"></i> {{ $t('uploadFolder') }}</el-dropdown-item>
                </el-dropdown-menu>
            </el-dropdown>
            <el-dropdown @command="handleNewCommand" size="mini" trigger="click">
                <el-button type="primary" size="mini" icon="el-icon-plus" :title="$t('createNew')"></el-button>
                <el-dropdown-menu slot="dropdown">
                    <el-dropdown-item command="folder"><i class="el-icon-folder-add"></i> {{ $t('newFolder') }}</el-dropdown-item>
                    <el-dropdown-item command="file"><i class="el-icon-document-add"></i> {{ $t('newFile') }}</el-dropdown-item>
                </el-dropdown-menu>
            </el-dropdown>
            <span class="drop-hint">{{ $t('dropToUpload') }}</span>
        </div>

        <!-- 工具行：批量模式 -->
        <div class="toolbar batch-bar" v-else>
            <span class="batch-count">{{ $t('selectedCount', { n: selectedNames.length }) }}</span>
            <el-button type="primary" size="mini" icon="el-icon-download" @click="batchDownload" :title="$t('download')"></el-button>
            <el-button type="danger" size="mini" icon="el-icon-delete" @click="batchDelete" :title="$t('delete')"></el-button>
            <el-button size="mini" icon="el-icon-close" @click="clearSelection" :title="$t('cancelSelect')"></el-button>
        </div>

        <div v-if="showSearchPanel" class="search-panel">
            <FileSearch @navigate="handleSearchNavigate" @open-file="handleSearchOpenFile" />
        </div>

        <!-- 列表表头（可点击排序） -->
        <div class="list-header">
            <div class="col-check"></div>
            <div class="col-name sortable" @click="toggleSort('name')">
                {{ $t('Name') }}<i :class="sortIcon('name')"></i>
            </div>
            <div class="col-size sortable" @click="toggleSort('size')">
                {{ $t('Size') }}<i :class="sortIcon('size')"></i>
            </div>
        </div>

        <!-- 文件列表 -->
        <div class="file-rows" v-loading="loading" :class="{ 'drag-over': dragOver }" @contextmenu="showBlankContextMenu">
            <div class="file-row" v-for="f in visibleFiles" :key="f.Name"
                :class="{ 'is-selected': isSelected(f), 'is-dir': f.IsDir }"
                @click="onRowClick(f, $event)" @dblclick="openEntry(f)"
                @contextmenu.stop.prevent="showRowContextMenu(f, $event)">
                <div class="col-check"><i class="el-icon-check" v-if="isSelected(f)"></i></div>
                <i class="file-icon" :class="fileMeta(f).icon" :style="{ color: fileMeta(f).color }"></i>
                <div class="file-main">
                    <div class="file-name" :title="f.Name">{{ f.Name }}</div>
                    <div class="file-meta">{{ fileMetaLine(f) }}</div>
                </div>
                <el-dropdown trigger="click" @command="(cmd) => handleFileCommand(cmd, f)" @click.native.stop>
                    <i class="el-icon-more row-more" :title="$t('Actions')" @click.stop></i>
                    <el-dropdown-menu slot="dropdown">
                        <el-dropdown-item command="open" v-if="!f.IsDir"><i class="el-icon-view"></i> {{ $t('open') }}</el-dropdown-item>
                        <el-dropdown-item command="download" v-if="!f.IsDir"><i class="el-icon-download"></i> {{ $t('download') }}</el-dropdown-item>
                        <el-dropdown-item command="rename"><i class="el-icon-edit-outline"></i> {{ $t('rename') }}</el-dropdown-item>
                        <el-dropdown-item command="copyPath"><i class="el-icon-document-copy"></i> {{ $t('copyPath') }}</el-dropdown-item>
                        <el-dropdown-item command="properties"><i class="el-icon-info"></i> {{ $t('properties') }}</el-dropdown-item>
                        <el-dropdown-item command="delete" divided><i class="el-icon-delete" style="color: #f56c6c;"></i> <span style="color: #f56c6c;">{{ $t('delete') }}</span></el-dropdown-item>
                    </el-dropdown-menu>
                </el-dropdown>
            </div>

            <!-- 空目录 / 无匹配 -->
            <div class="empty-state" v-if="!loading && visibleFiles.length === 0">
                <i class="el-icon-folder-opened"></i>
                <div>{{ $t('emptyDirectory') }}</div>
                <div class="empty-tip">{{ $t('dropToUpload') }}</div>
            </div>
        </div>

        <!-- 拖拽上传遮罩提示 -->
        <div class="drag-mask" v-if="dragOver">
            <i class="el-icon-upload"></i>
            <div>{{ $t('dropToUpload') }}</div>
        </div>

        <!-- 拖拽上传进度 -->
        <div class="upload-queue" v-if="uploads.length > 0">
            <div class="upload-queue-title">{{ $t('uploading') }}<i class="el-icon-close" @click="uploads = []"></i></div>
            <div class="upload-item" v-for="u in uploads" :key="u.id">
                <div class="upload-item-name" :title="u.name">{{ u.name }}</div>
                <el-progress :percentage="u.percent" :status="u.status === 'error' ? 'exception' : (u.percent >= 100 ? 'success' : undefined)" :stroke-width="6"></el-progress>
            </div>
        </div>

        <!-- 右键菜单：文件行 -->
        <div ref="contextMenu" v-show="contextMenuVisible" class="context-menu" :style="{ left: contextMenuX + 'px', top: contextMenuY + 'px' }">
            <template v-if="contextMenuRow">
                <div class="context-menu-item" @click="handleContextCommand('open')" v-if="!contextMenuRow.IsDir">
                    <i class="el-icon-view"></i> {{ $t('open') }}
                </div>
                <div class="context-menu-item" @click="handleContextCommand('download')" v-if="!contextMenuRow.IsDir">
                    <i class="el-icon-download"></i> {{ $t('download') }}
                </div>
                <div class="context-menu-item" @click="handleContextCommand('rename')">
                    <i class="el-icon-edit-outline"></i> {{ $t('rename') }}
                </div>
                <div class="context-menu-item" @click="handleContextCommand('copyPath')">
                    <i class="el-icon-document-copy"></i> {{ $t('copyPath') }}
                </div>
                <div class="context-menu-item" @click="handleContextCommand('properties')">
                    <i class="el-icon-info"></i> {{ $t('properties') }}
                </div>
                <div class="context-menu-item context-danger" @click="handleContextCommand('delete')">
                    <i class="el-icon-delete"></i> {{ $t('delete') }}
                </div>
            </template>
            <template v-else>
                <div class="context-menu-item" @click="handleContextCommand('refresh')">
                    <i class="el-icon-refresh"></i> {{ $t('refresh') }}
                </div>
                <div class="context-menu-item" @click="handleContextCommand('upload')">
                    <i class="el-icon-upload"></i> {{ $t('uploadFile') }}
                </div>
                <div class="context-menu-item" @click="handleContextCommand('newFolder')">
                    <i class="el-icon-folder-add"></i> {{ $t('newFolder') }}
                </div>
                <div class="context-menu-item" @click="handleContextCommand('newFile')">
                    <i class="el-icon-document-add"></i> {{ $t('newFile') }}
                </div>
                <div class="context-menu-item" @click="handleContextCommand('toggleHidden')">
                    <i class="el-icon-view"></i> {{ showHidden ? $t('hideHiddenFiles') : $t('showHiddenFiles') }}
                </div>
                <div class="context-menu-item" @click="handleContextCommand('selectAll')">
                    <i class="el-icon-finished"></i> {{ $t('selectAll') }}
                </div>
            </template>
        </div>

        <!-- 属性抽屉 -->
        <el-drawer custom-class="file-info-drawer" :visible.sync="infoVisible" :with-header="false" append-to-body size="300px">
            <div class="file-info" v-loading="infoLoading">
                <div class="file-info-icon"><i :class="infoData ? fileMeta({ Name: infoData.name, IsDir: infoData.isDir }).icon : 'el-icon-document'" :style="{ color: infoData ? fileMeta({ Name: infoData.name, IsDir: infoData.isDir }).color : '' }"></i></div>
                <div class="file-info-name" :title="infoData ? infoData.name : ''">{{ infoData ? infoData.name : '' }}</div>
                <div class="file-info-path" :title="infoPath">{{ infoPath }}</div>
                <div class="file-info-row" v-for="row in infoRows" :key="row.label">
                    <span class="label">{{ row.label }}</span>
                    <span class="value">{{ row.value }}</span>
                </div>
            </div>
        </el-drawer>

        <!-- 上传对话框（工具栏入口） -->
        <el-dialog custom-class="uploadContainer" :title="$t(this.titleTip)" :visible.sync="uploadVisible" append-to-body width="32%">
            <el-upload ref="upload" multiple drag :action="uploadUrl" :data="uploadData" :before-upload="beforeUpload" :on-progress="uploadProgress" :on-success="uploadSuccess">
                <i class="el-icon-upload"></i>
                <div class="el-upload__text">{{ $t(this.selectTip) }}</div>
                <div class="el-upload__tip" slot="tip">{{ this.uploadTip }}</div>
            </el-upload>
        </el-dialog>

        <!-- 新建文件夹 / 新建文件 / 重命名 -->
        <el-dialog :title="$t('newFolder')" :visible.sync="newFolderVisible" append-to-body width="30%">
            <el-input v-model="newFolderName" :placeholder="$t('enterFolderName')" @keyup.enter.native="createNewFolder"></el-input>
            <span slot="footer" class="dialog-footer">
                <el-button @click="newFolderVisible = false">{{ $t('Cancel') }}</el-button>
                <el-button type="primary" @click="createNewFolder">{{ $t('OK') }}</el-button>
            </span>
        </el-dialog>

        <el-dialog :title="$t('newFile')" :visible.sync="newFileVisible" append-to-body width="30%">
            <el-input v-model="newFileName" :placeholder="$t('enterFileName')" @keyup.enter.native="createNewFile"></el-input>
            <span slot="footer" class="dialog-footer">
                <el-button @click="newFileVisible = false">{{ $t('Cancel') }}</el-button>
                <el-button type="primary" @click="createNewFile">{{ $t('OK') }}</el-button>
            </span>
        </el-dialog>

        <el-dialog :title="$t('rename')" :visible.sync="renameDialogVisible" append-to-body width="30%">
            <el-input v-model="newName" :placeholder="$t('enterNewName')" @keyup.enter.native="confirmRename"></el-input>
            <span slot="footer" class="dialog-footer">
                <el-button @click="renameDialogVisible = false">{{ $t('Cancel') }}</el-button>
                <el-button type="primary" @click="confirmRename">{{ $t('OK') }}</el-button>
            </span>
        </el-dialog>

        <FileEditor v-model="editorVisible" :filePath="editingFilePath" @saved="getFileList" />
    </div>
</template>

<script>
import { fileList, deleteFile, renameFile, createFolder, saveFile, getFileInfo } from '@/api/file'
import { mapState } from 'vuex'
import FileEditor from './FileEditor.vue'
import FileSearch from './FileSearch.vue'

// 文本类扩展名：双击直接进编辑器，其余双击下载
const TEXT_EXTS = ['txt', 'md', 'json', 'xml', 'yaml', 'yml', 'toml', 'ini', 'conf', 'cfg', 'log', 'csv',
    'go', 'py', 'js', 'ts', 'jsx', 'tsx', 'vue', 'html', 'htm', 'css', 'scss', 'less', 'java', 'c', 'cpp',
    'h', 'hpp', 'rs', 'rb', 'php', 'sh', 'bash', 'zsh', 'sql', 'lua', 'pl', 'env', 'properties', 'service']
const IMAGE_EXTS = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'ico']
const VIDEO_EXTS = ['mp4', 'mkv', 'avi', 'mov', 'webm', 'flv']
const AUDIO_EXTS = ['mp3', 'wav', 'flac', 'ogg', 'm4a', 'aac']
const ARCHIVE_EXTS = ['zip', 'tar', 'gz', 'tgz', 'bz2', 'xz', '7z', 'rar', 'jar', 'war']

function extOf(name) {
    const idx = name.lastIndexOf('.')
    return idx > 0 ? name.slice(idx + 1).toLowerCase() : ''
}

export default {
    name: 'FileList',
    components: { FileEditor, FileSearch },
    data() {
        return {
            uploadVisible: false,
            fileList: [],
            currentPath: '/',
            selectTip: 'clickSelectFile',
            titleTip: 'uploadFile',
            uploadTip: '',
            initialRedirectDone: false,
            homePath: '',
            showSearchPanel: false,
            loading: false,
            // 视图状态
            showHidden: localStorage.getItem('sftpShowHidden') !== 'false',
            sortKey: 'name',          // name | size | time
            sortAsc: true,
            // 选择
            selectedNames: [],
            // 路径编辑
            pathEditing: false,
            pathDraft: '/',
            // 新建 / 重命名
            newFolderVisible: false,
            newFolderName: '',
            newFileVisible: false,
            newFileName: '',
            renameDialogVisible: false,
            newName: '',
            renamingRow: null,
            // 右键菜单
            contextMenuVisible: false,
            contextMenuX: 0,
            contextMenuY: 0,
            contextMenuRow: null,
            // 编辑器
            editorVisible: false,
            editingFilePath: '',
            // 属性
            infoVisible: false,
            infoLoading: false,
            infoData: null,
            infoPath: '',
            // 拖拽上传
            dragOver: false,
            dragCounter: 0,
            uploads: [],
            uploadQueue: [],
            uploadingBusy: false
        }
    },
    mounted() {
        if (!this.currentPath || this.currentPath === '/') {
            this.getFileList()
        }
        document.addEventListener('click', this.hideContextMenu)
        document.addEventListener('keydown', this.onKeydown)
    },
    beforeDestroy() {
        document.removeEventListener('click', this.hideContextMenu)
        document.removeEventListener('keydown', this.onKeydown)
    },
    computed: {
        ...mapState(['currentTab']),
        sshInfoReady() {
            return this.$store.state.sshInfo && this.$store.state.sshInfo.hostname;
        },
        uploadUrl: () => {
            return `${process.env.NODE_ENV === 'production' ? `${location.origin}` : 'api'}/file/upload`
        },
        uploadData: function() {
            return {
                sshInfo: this.$store.getters.sshReq,
                path: this.currentPath
            }
        },
        // 隐藏文件过滤 + 排序（目录永远在前）
        visibleFiles() {
            let list = this.fileList
            if (!this.showHidden) {
                list = list.filter(f => !f.Name.startsWith('.'))
            }
            const dirFirst = (a, b) => (a.IsDir === b.IsDir ? 0 : (a.IsDir ? -1 : 1))
            const key = this.sortKey, asc = this.sortAsc ? 1 : -1
            return [...list].sort((a, b) => {
                const d = dirFirst(a, b)
                if (d !== 0) return d
                let r = 0
                if (key === 'size') {
                    r = (a.SizeBytes || 0) - (b.SizeBytes || 0)
                } else if (key === 'time') {
                    r = String(a.ModifyTime).localeCompare(String(b.ModifyTime))
                } else {
                    r = String(a.Name).localeCompare(String(b.Name), 'zh-Hans-CN', { numeric: true })
                }
                return r * asc
            })
        },
        crumbs() {
            const path = this.currentPath || '/'
            const parts = path.split('/').filter(p => p !== '')
            const crumbs = [{ name: '/', path: '/' }]
            let acc = ''
            for (const p of parts) {
                acc += '/' + p
                crumbs.push({ name: p, path: acc })
            }
            return crumbs
        },
        sortValue() {
            return `${this.sortKey}-${this.sortAsc ? 'asc' : 'desc'}`
        },
        sortOptions() {
            return [
                { value: 'name-asc', label: `${this.$t('Name')} ↑` },
                { value: 'name-desc', label: `${this.$t('Name')} ↓` },
                { value: 'size-asc', label: `${this.$t('Size')} ↑` },
                { value: 'size-desc', label: `${this.$t('Size')} ↓` },
                { value: 'time-asc', label: `${this.$t('ModifiedTime')} ↑` },
                { value: 'time-desc', label: `${this.$t('ModifiedTime')} ↓` }
            ]
        },
        infoRows() {
            if (!this.infoData) return []
            const dir = this.infoData.isDir
            return [
                { label: this.$t('type'), value: dir ? this.$t('dirLabel') : this.$t('fileLabel') },
                { label: this.$t('Size'), value: dir ? '-' : String(this.infoData.size || '-') },
                { label: this.$t('ModifiedTime'), value: this.infoData.modTime || '-' },
                { label: this.$t('editorType'), value: dir ? '-' : (this.infoData.isText ? (this.infoData.language || 'text') : this.$t('binaryFile')) }
            ]
        }
    },
    watch: {
        sshInfoReady(newValue, oldValue) {
            if (newValue && !oldValue) {
                this.getFileList();
            }
        },
        currentTab: function() {
            this.fileList = []
            this.clearSelection()
            this.currentPath = this.currentTab && this.currentTab.path ? this.currentTab.path : '/';
        },
        showHidden(v) {
            localStorage.setItem('sftpShowHidden', String(v))
        }
    },
    methods: {
        /* ---------- 导航 ---------- */
        navigateTo(path) {
            if (path === this.currentPath) return
            this.clearSelection()
            this.currentPath = path
            this.getFileList()
        },
        goToHome() {
            if (this.homePath) {
                if (this.currentPath !== this.homePath) {
                    this.navigateTo(this.homePath)
                }
            } else {
                this.$message.warning(this.$t('homeNotReady'));
            }
        },
        upDirectory() {
            if (this.currentPath === '/') return
            const idx = this.currentPath.lastIndexOf('/')
            const parent = idx <= 0 ? '/' : this.currentPath.slice(0, idx)
            this.navigateTo(parent)
        },
        startPathEdit() {
            this.pathDraft = this.currentPath
            this.pathEditing = true
            this.$nextTick(() => {
                const input = this.$refs.pathInput
                if (input) {
                    input.focus()
                    input.select && input.select()
                }
            })
        },
        commitPathEdit() {
            if (!this.pathEditing) return
            this.pathEditing = false
            let target = (this.pathDraft || '').trim().replace(/\\+/g, '/')
            if (!target) target = '/'
            if (target !== this.currentPath) {
                this.navigateTo(target)
            }
        },
        copyPath(path) {
            this.copyText(path)
        },
        copyText(text) {
            const done = () => this.$message.success(this.$t('copySuccess'))
            const fail = () => this.$message.error(this.$t('copyFail'))
            if (navigator.clipboard && window.isSecureContext) {
                navigator.clipboard.writeText(text).then(done).catch(() => this.fallbackCopy(text) ? done() : fail())
            } else {
                this.fallbackCopy(text) ? done() : fail()
            }
        },
        fallbackCopy(text) {
            try {
                const ta = document.createElement('textarea')
                ta.value = text
                ta.style.position = 'fixed'
                ta.style.opacity = '0'
                document.body.appendChild(ta)
                ta.select()
                const ok = document.execCommand('copy')
                document.body.removeChild(ta)
                return ok
            } catch (e) {
                return false
            }
        },

        /* ---------- 列表加载 ---------- */
        async getFileList() {
            this.loading = true
            this.currentPath = this.currentPath.replace(/\\+/g, '/')
            if (this.currentPath === '') {
                this.currentPath = '/'
            }
            if (!this.currentPath.startsWith('/')) {
                this.currentPath = '/' + this.currentPath
            }
            this.$store.commit('SET_CURRENT_PATH', this.currentPath)
            try {
                const result = await fileList(this.currentPath, this.$store.getters.sshReq)
                if (result.Msg === 'success') {
                    if (result.Data.home) {
                        this.homePath = result.Data.home;
                    }
                    this.fileList = result.Data.list === null ? [] : result.Data.list
                    this.clearSelection()
                    if (!this.initialRedirectDone && result.Data.home && result.Data.home !== '/' && this.currentPath !== result.Data.home) {
                        this.initialRedirectDone = true
                        this.currentPath = result.Data.home
                        this.loading = false
                        await this.getFileList()
                        return
                    }
                } else {
                    this.fileList = []
                    this.$message({
                        message: result.Msg,
                        type: 'error',
                        duration: 3000
                    })
                }
            } catch (e) {
                this.fileList = []
                this.$message.error(this.$t('loadFileError'))
            } finally {
                this.loading = false
            }
        },

        /* ---------- 选择 ---------- */
        isSelected(f) {
            return this.selectedNames.indexOf(f.Name) !== -1
        },
        onRowClick(f, event) {
            if (event.ctrlKey || event.metaKey) {
                const idx = this.selectedNames.indexOf(f.Name)
                if (idx === -1) {
                    this.selectedNames.push(f.Name)
                } else {
                    this.selectedNames.splice(idx, 1)
                }
            } else {
                this.selectedNames = this.isSelected(f) && this.selectedNames.length === 1 ? [] : [f.Name]
            }
        },
        clearSelection() {
            this.selectedNames = []
        },
        selectAllVisible() {
            this.selectedNames = this.visibleFiles.map(f => f.Name)
        },

        /* ---------- 打开 / 下载 ---------- */
        joinPath(name) {
            return this.currentPath.endsWith('/') ? this.currentPath + name : this.currentPath + '/' + name
        },
        openEntry(f) {
            if (f.IsDir) {
                this.navigateTo(this.joinPath(f.Name))
                return
            }
            const ext = extOf(f.Name)
            if (TEXT_EXTS.indexOf(ext) !== -1 || f.Name.startsWith('.')) {
                this.handleFileCommand('edit', f)
            } else {
                this.handleFileCommand('download', f)
            }
        },
        downloadFile(path) {
            const prefix = process.env.NODE_ENV === 'production' ? `${location.origin}` : 'api'
            const downloadUrl = `${prefix}/file/download?path=${encodeURIComponent(path)}&sshInfo=${this.$store.getters.sshReq}`
            // 用隐藏锚点触发下载，避免 window.open 被弹窗拦截
            const a = document.createElement('a')
            a.href = downloadUrl
            a.rel = 'noopener'
            document.body.appendChild(a)
            a.click()
            document.body.removeChild(a)
        },

        /* ---------- 排序 ---------- */
        sortIcon(key) {
            if (this.sortKey !== key) return 'el-icon-d-caret sort-idle'
            return this.sortAsc ? 'el-icon-caret-top' : 'el-icon-caret-bottom'
        },
        toggleSort(key) {
            if (this.sortKey === key) {
                this.sortAsc = !this.sortAsc
            } else {
                this.sortKey = key
                this.sortAsc = true
            }
        },
        handleSortCommand(value) {
            const [key, dir] = value.split('-')
            this.sortKey = key
            this.sortAsc = dir === 'asc'
        },

        /* ---------- 上传（工具栏对话框） ---------- */
        openUploadDialog() {
            this.uploadTip = `${this.$t('uploadPath')}: ${this.currentPath}`
            this.uploadVisible = true
        },
        handleUploadCommand(cmd) {
            if (cmd === 'folder') {
                this.selectTip = 'clickSelectFolder'
                this.titleTip = 'uploadFolder'
            } else {
                this.selectTip = 'clickSelectFile'
                this.titleTip = 'uploadFile'
            }
            this.openUploadDialog();
            const isFolder = 'folder' === cmd,
                supported = this.webkitdirectorySupported();
            if (!supported) {
                isFolder && this.$message.warning(this.$t('browserNotSupported'));
                return;
            }
            this.$nextTick(() => {
                const input = document.getElementsByClassName('el-upload__input')[0];
                if (input) input.webkitdirectory = isFolder;
            })
        },
        webkitdirectorySupported(){
            return 'webkitdirectory' in document.createElement('input')
        },
        beforeUpload(file) {
            this.uploadTip = `${this.$t('uploading')} ${file.name} ${this.$t('to')} ${this.currentPath}, ${this.$t('notCloseWindows')}..`
            this.uploadData.id = file.uid
            const dirPath = file.webkitRelativePath;
            this.uploadData.dir = dirPath ? dirPath.substring(0, dirPath.lastIndexOf('/')) : '';
            return true
        },
        uploadSuccess(r, file) {
            this.uploadTip = `${file.name}${this.$t('uploadFinish')}!`
            this.getFileList();
        },
        uploadProgress(e, f) {
            e.percent = e.percent / 2
            f.percentage = f.percentage / 2
            if (e.percent === 50) {
                const ws = new WebSocket(`${(location.protocol === 'http:' ? 'ws' : 'wss')}://${location.host}${process.env.NODE_ENV === 'production' ? '' : '/ws'}/file/progress?id=${f.uid}`)
                ws.onmessage = e1 => {
                    f.percentage = (f.size + Number(e1.data)) / (f.size * 2) * 100
                }
                ws.onclose = () => { /* noop */ }
                ws.onerror = () => { /* noop */ }
            }
        },

        /* ---------- 拖拽上传（独立于对话框，串行队列 + 双段进度） ---------- */
        onDragEnter() {
            this.dragCounter++
            this.dragOver = true
        },
        onDragLeave() {
            this.dragCounter--
            if (this.dragCounter <= 0) {
                this.dragCounter = 0
                this.dragOver = false
            }
        },
        onDrop(e) {
            this.dragCounter = 0
            this.dragOver = false
            const files = Array.from((e.dataTransfer && e.dataTransfer.files) || [])
            if (files.length === 0) {
                this.$message.info(this.$t('dropFilesOnly'))
                return
            }
            for (const file of files) {
                this.uploadQueue.push({
                    id: 'drop-' + Date.now() + '-' + Math.random().toString(36).slice(2, 8),
                    file
                })
            }
            this.processUploadQueue()
        },
        processUploadQueue() {
            if (this.uploadingBusy) return
            const job = this.uploadQueue.shift()
            if (!job) return
            this.uploadingBusy = true
            const item = { id: job.id, name: job.file.name, percent: 0, status: 'uploading' }
            this.uploads.push(item)
            const wsBase = `${(location.protocol === 'http:' ? 'ws' : 'wss')}://${location.host}${process.env.NODE_ENV === 'production' ? '' : '/ws'}/file/progress?id=${job.id}`
            let ws = null
            let wsOpened = false
            const formData = new FormData()
            formData.append('sshInfo', this.$store.getters.sshReq)
            formData.append('path', this.currentPath)
            formData.append('id', job.id)
            formData.append('dir', '')
            formData.append('file', job.file)

            const finish = ok => {
                try { if (ws) ws.close() } catch (err) { /* noop */ }
                item.percent = 100
                item.status = ok ? 'success' : 'error'
                if (ok) this.getFileList()
                this.uploadingBusy = false
                this.processUploadQueue()
            }

            const xhr = new XMLHttpRequest()
            xhr.open('POST', this.uploadUrl, true)
            xhr.upload.onprogress = ev => {
                if (!ev.lengthComputable) return
                // 0~50%：浏览器 -> 服务端；到达 50% 后打开 WS 跟踪服务端 -> 远端主机
                item.percent = Math.min(49, Math.floor(ev.loaded / ev.total * 50))
                if (!wsOpened && ev.loaded >= ev.total) {
                    wsOpened = true
                    try {
                        ws = new WebSocket(wsBase)
                        ws.onmessage = e1 => {
                            const written = Number(e1.data)
                            item.percent = Math.min(99, Math.floor(50 + written / (job.file.size || 1) * 50))
                        }
                        ws.onclose = () => { /* noop */ }
                        ws.onerror = () => { /* noop */ }
                    } catch (err) { /* noop */ }
                }
            }
            xhr.onload = () => {
                // /file/upload 在远端写入完成后才响应，onload 即代表整个上传结束
                let ok = false
                let msg = String(xhr.status)
                try {
                    const resp = JSON.parse(xhr.responseText)
                    ok = xhr.status === 200 && resp.Msg === 'success'
                    if (!ok) msg = resp.Msg || msg
                } catch (err) { /* noop */ }
                if (!ok) this.$message.error(`${job.file.name}: ${msg}`)
                finish(ok)
            }
            xhr.onerror = () => {
                this.$message.error(this.$t('uploadFail'))
                finish(false)
            }
            xhr.send(formData)
        },

        /* ---------- 新建 / 重命名 / 删除 ---------- */
        handleNewCommand(cmd) {
            if (cmd === 'folder') {
                this.newFolderName = ''
                this.newFolderVisible = true
            } else {
                this.newFileName = ''
                this.newFileVisible = true
            }
        },
        async createNewFolder() {
            if (!this.newFolderName.trim()) {
                this.$message.warning(this.$t('enterFolderName'))
                return
            }
            try {
                const result = await createFolder(this.joinPath(this.newFolderName.trim()), this.$store.getters.sshReq)
                if (result.Msg === 'success') {
                    this.$message.success(this.$t('folderCreated'))
                    this.newFolderVisible = false
                    this.getFileList()
                } else {
                    this.$message.error(result.Msg)
                }
            } catch (e) {
                this.$message.error(this.$t('createFolderError'))
            }
        },
        async createNewFile() {
            if (!this.newFileName.trim()) {
                this.$message.warning(this.$t('enterFileName'))
                return
            }
            try {
                // /file/save 基于 sftp Create，写入空内容即创建空文件
                const result = await saveFile(this.joinPath(this.newFileName.trim()), '', this.$store.getters.sshReq)
                if (result.Msg === 'success') {
                    this.$message.success(this.$t('fileCreated'))
                    this.newFileVisible = false
                    this.getFileList()
                } else {
                    this.$message.error(result.Msg)
                }
            } catch (e) {
                this.$message.error(this.$t('createFileError'))
            }
        },
        async confirmRename() {
            if (!this.newName.trim() || !this.renamingRow) {
                this.$message.warning(this.$t('enterNewName'))
                return
            }
            try {
                const result = await renameFile(this.joinPath(this.renamingRow.Name), this.joinPath(this.newName.trim()), this.$store.getters.sshReq)
                if (result.Msg === 'success') {
                    this.$message.success(this.$t('renameSuccess'))
                    this.renameDialogVisible = false
                    this.getFileList()
                } else {
                    this.$message.error(result.Msg)
                }
            } catch (e) {
                this.$message.error(this.$t('renameError'))
            }
        },
        confirmDelete(path, name) {
            this.$confirm(this.$t('deleteConfirm', { name }), this.$t('warning'), {
                confirmButtonText: this.$t('OK'),
                cancelButtonText: this.$t('Cancel'),
                type: 'warning'
            }).then(async () => {
                try {
                    const result = await deleteFile(path, this.$store.getters.sshReq)
                    if (result.Msg === 'success') {
                        this.$message.success(this.$t('deleteSuccess'))
                        this.getFileList()
                    } else {
                        this.$message.error(result.Msg)
                    }
                } catch (e) {
                    this.$message.error(this.$t('deleteError'))
                }
            }).catch(() => {})
        },

        /* ---------- 批量操作 ---------- */
        selectedRows() {
            return this.fileList.filter(f => this.selectedNames.indexOf(f.Name) !== -1)
        },
        batchDownload() {
            const files = this.selectedRows().filter(f => !f.IsDir)
            const dirs = this.selectedRows().length - files.length
            if (files.length === 0) {
                this.$message.info(this.$t('batchDownloadSkipDirs'))
                return
            }
            if (dirs > 0) {
                this.$message.info(this.$t('batchDownloadSkipDirs'))
            }
            files.forEach((f, i) => {
                setTimeout(() => this.downloadFile(this.joinPath(f.Name)), i * 300)
            })
        },
        batchDelete() {
            const rows = this.selectedRows()
            if (rows.length === 0) return
            this.$confirm(this.$t('batchDeleteConfirm', { n: rows.length }), this.$t('warning'), {
                confirmButtonText: this.$t('OK'),
                cancelButtonText: this.$t('Cancel'),
                type: 'warning'
            }).then(async () => {
                let failed = 0
                for (const f of rows) {
                    try {
                        const result = await deleteFile(this.joinPath(f.Name), this.$store.getters.sshReq)
                        if (result.Msg !== 'success') failed++
                    } catch (e) {
                        failed++
                    }
                }
                if (failed === 0) {
                    this.$message.success(this.$t('deleteSuccess'))
                } else {
                    this.$message.error(this.$t('batchDeletePartial', { n: failed }))
                }
                this.clearSelection()
                this.getFileList()
            }).catch(() => {})
        },

        /* ---------- 右键菜单 ---------- */
        showRowContextMenu(f, event) {
            this.contextMenuRow = f
            this.showContextMenuAt(event)
            // 右键同时选中该行，行为可预期
            if (!this.isSelected(f)) {
                this.selectedNames = [f.Name]
            }
        },
        showBlankContextMenu(event) {
            this.contextMenuRow = null
            this.showContextMenuAt(event)
        },
        showContextMenuAt(event) {
            const menuWidth = 150
            const menuHeight = 210
            this.contextMenuX = Math.min(event.clientX, window.innerWidth - menuWidth - 8)
            this.contextMenuY = Math.min(event.clientY, window.innerHeight - menuHeight - 8)
            this.contextMenuVisible = true
        },
        hideContextMenu() {
            this.contextMenuVisible = false
        },
        handleContextCommand(cmd) {
            const row = this.contextMenuRow
            this.hideContextMenu()
            if (!row) {
                switch (cmd) {
                    case 'refresh': this.getFileList(); break
                    case 'upload': this.handleUploadCommand('file'); break
                    case 'newFolder': this.handleNewCommand('folder'); break
                    case 'newFile': this.handleNewCommand('file'); break
                    case 'toggleHidden': this.showHidden = !this.showHidden; break
                    case 'selectAll': this.selectAllVisible(); break
                }
                return
            }
            this.handleFileCommand(cmd, row)
        },
        handleFileCommand(cmd, row) {
            const path = this.joinPath(row.Name)
            switch (cmd) {
                case 'open':
                    this.openEntry(row)
                    break
                case 'edit':
                    this.editingFilePath = path
                    this.editorVisible = true
                    break
                case 'download':
                    this.downloadFile(path)
                    break
                case 'rename':
                    this.renamingRow = row
                    this.newName = row.Name
                    this.renameDialogVisible = true
                    break
                case 'copyPath':
                    this.copyText(path)
                    break
                case 'properties':
                    this.showProperties(path)
                    break
                case 'delete':
                    this.confirmDelete(path, row.Name)
                    break
            }
        },

        /* ---------- 属性 ---------- */
        async showProperties(path) {
            this.infoPath = path
            this.infoData = null
            this.infoVisible = true
            this.infoLoading = true
            try {
                const result = await getFileInfo(path, this.$store.getters.sshReq)
                if (result.Msg === 'success') {
                    this.infoData = result.Data
                } else {
                    this.$message.error(result.Msg)
                    this.infoVisible = false
                }
            } catch (e) {
                this.$message.error(this.$t('infoLoadError'))
                this.infoVisible = false
            } finally {
                this.infoLoading = false
            }
        },

        /* ---------- 展示辅助 ---------- */
        fileMeta(f) {
            if (f.IsDir) return { icon: 'el-icon-folder', color: '#e6a23c' }
            const ext = extOf(f.Name)
            if (IMAGE_EXTS.indexOf(ext) !== -1) return { icon: 'el-icon-picture-outline', color: '#67c23a' }
            if (VIDEO_EXTS.indexOf(ext) !== -1) return { icon: 'el-icon-film', color: '#f56c6c' }
            if (AUDIO_EXTS.indexOf(ext) !== -1) return { icon: 'el-icon-headset', color: '#f56c6c' }
            if (ARCHIVE_EXTS.indexOf(ext) !== -1) return { icon: 'el-icon-tickets', color: '#9093ff' }
            if (TEXT_EXTS.indexOf(ext) !== -1 || f.Name.startsWith('.')) return { icon: 'el-icon-document', color: '#409eff' }
            return { icon: 'el-icon-document', color: '#909399' }
        },
        fileMetaLine(f) {
            if (f.IsDir) return `${this.$t('dirLabel')} · ${String(f.ModifyTime || '').slice(5, 16)}`
            return `${f.Size} · ${String(f.ModifyTime || '').slice(5, 16)}`
        },

        /* ---------- 全局键盘 ---------- */
        onKeydown(e) {
            if (e.key === 'Escape') {
                this.hideContextMenu()
                this.clearSelection()
            }
        },

        /* ---------- 搜索面板 ---------- */
        handleSearchNavigate(path) {
            this.navigateTo(path)
            this.showSearchPanel = false
        },
        handleSearchOpenFile(path) {
            this.editingFilePath = path
            this.editorVisible = true
            this.showSearchPanel = false
        }
    }
}
</script>

<style lang="scss">
.file-list-wrapper {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding-top: 10px;
    box-sizing: border-box;
    position: relative;

    .sftp-title {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 15px;
        font-weight: bold;
        color: var(--text-color);
        padding-bottom: 8px;
        margin-bottom: 8px;
        border-bottom: 1px solid var(--input-border);
        flex-shrink: 0;

        .entry-count {
            font-size: 12px;
            font-weight: normal;
            opacity: 0.65;
        }

        .title-actions {
            margin-left: auto;
            display: flex;
            align-items: center;
            gap: 10px;

            .title-action {
                cursor: pointer;
                font-size: 15px;
                color: var(--text-color);
                opacity: 0.65;
                transition: opacity 0.2s, color 0.2s;

                &:hover {
                    opacity: 1;
                    color: var(--primary);
                }

                &.is-active {
                    opacity: 1;
                    color: var(--primary);
                }
            }
        }
    }

    .path-row {
        flex-shrink: 0;
        display: flex;
        align-items: center;
        gap: 4px;
        margin-bottom: 6px;

        .breadcrumb {
            flex: 1;
            min-width: 0;
            height: 28px;
            line-height: 28px;
            border: 1px solid var(--input-border);
            border-radius: 4px;
            background: var(--input-bg);
            padding: 0 8px;
            cursor: text;
            overflow: hidden;

            /* RTL 技巧：路径过长时省略左侧（保留末级目录可见） */
            direction: rtl;
            text-align: left;

            .crumb-inner {
                direction: ltr;
                display: inline-block;
                white-space: nowrap;
            }

            .crumb {
                cursor: pointer;
                font-size: 13px;
                color: var(--text-color);
                padding: 0 2px;

                &:hover {
                    color: var(--primary);
                    text-decoration: underline;
                }

                &::after {
                    content: '/';
                    margin: 0 3px;
                    opacity: 0.4;
                }

                &:last-child::after {
                    content: '';
                }
            }
        }

        .path-action {
            cursor: pointer;
            font-size: 15px;
            color: var(--text-color);
            opacity: 0.6;

            &:hover {
                opacity: 1;
                color: var(--primary);
            }
        }
    }

    .toolbar {
        flex-shrink: 0;
        display: flex;
        align-items: center;
        gap: 6px;
        margin-bottom: 8px;
        flex-wrap: nowrap;

        .el-button-group .el-button {
            padding: 7px 9px;
        }

        .el-button {
            padding: 7px 9px;
        }

        .is-plain-active {
            background: var(--primary);
            border-color: var(--primary);
        }

        .drop-hint {
            margin-left: auto;
            font-size: 11px;
            opacity: 0.5;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        &.batch-bar {
            .batch-count {
                font-size: 13px;
                color: var(--text-color);
                margin-right: 2px;
                white-space: nowrap;
            }
        }
    }

    .search-panel {
        flex-shrink: 0;
        max-height: 300px;
        overflow: hidden;
        border: 1px solid var(--input-border);
        border-radius: 4px;
        margin-bottom: 10px;
    }

    .list-header {
        flex-shrink: 0;
        display: flex;
        align-items: center;
        height: 28px;
        padding: 0 8px;
        font-size: 12px;
        color: var(--text-color);
        opacity: 0.75;
        border-bottom: 1px solid var(--input-border);

        .col-check {
            width: 18px;
            flex-shrink: 0;
        }

        .col-name {
            flex: 1;
            min-width: 0;
        }

        .col-size {
            width: 56px;
            flex-shrink: 0;
            text-align: right;
        }

        .sortable {
            cursor: pointer;
            display: flex;
            align-items: center;
            gap: 2px;
            user-select: none;

            &:hover {
                color: var(--primary);
            }

            i {
                font-size: 12px;
            }

            .sort-idle {
                opacity: 0.35;
            }
        }
    }

    .file-rows {
        flex: 1;
        min-height: 0;
        overflow-y: auto;
        position: relative;
        border-radius: 4px;

        &.drag-over {
            outline: 2px dashed var(--primary);
            outline-offset: -2px;
        }

        .file-row {
            display: flex;
            align-items: center;
            gap: 6px;
            padding: 5px 8px;
            cursor: pointer;
            border-bottom: 1px solid transparent;
            user-select: none;

            &:hover {
                background: var(--hover-bg, rgba(64, 158, 255, 0.08));
            }

            &.is-selected {
                background: rgba(64, 158, 255, 0.16);
                box-shadow: inset 2px 0 0 var(--primary);
            }

            .col-check {
                width: 14px;
                flex-shrink: 0;
                text-align: center;

                i {
                    color: var(--primary);
                    font-weight: bold;
                }
            }

            .file-icon {
                flex-shrink: 0;
                font-size: 17px;
            }

            .file-main {
                flex: 1;
                min-width: 0;
                line-height: 1.3;

                .file-name {
                    font-size: 13px;
                    color: var(--text-color);
                    white-space: nowrap;
                    overflow: hidden;
                    text-overflow: ellipsis;
                }

                .file-meta {
                    font-size: 11px;
                    opacity: 0.55;
                    white-space: nowrap;
                    overflow: hidden;
                    text-overflow: ellipsis;
                }
            }

            .row-more {
                flex-shrink: 0;
                font-size: 15px;
                color: var(--text-color);
                opacity: 0;
                padding: 2px 4px;
                transition: opacity 0.15s;

                &:hover {
                    color: var(--primary);
                    opacity: 1;
                }
            }

            &:hover .row-more {
                opacity: 0.75;
            }
        }

        .empty-state {
            text-align: center;
            padding: 40px 0 30px;
            color: var(--text-color);
            opacity: 0.45;

            i {
                font-size: 38px;
                margin-bottom: 8px;
            }

            div {
                font-size: 13px;
            }

            .empty-tip {
                font-size: 11px;
                margin-top: 4px;
                opacity: 0.8;
            }
        }
    }

    .drag-mask {
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        z-index: 20;
        background: rgba(64, 158, 255, 0.12);
        border: 2px dashed var(--primary);
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        color: var(--primary);
        pointer-events: none;

        i {
            font-size: 42px;
            margin-bottom: 8px;
        }
    }

    .upload-queue {
        flex-shrink: 0;
        border: 1px solid var(--input-border);
        border-radius: 4px;
        padding: 6px 8px;
        margin-top: 6px;
        max-height: 140px;
        overflow-y: auto;

        .upload-queue-title {
            display: flex;
            align-items: center;
            justify-content: space-between;
            font-size: 12px;
            font-weight: bold;
            color: var(--text-color);
            margin-bottom: 4px;

            i {
                cursor: pointer;
                opacity: 0.6;

                &:hover {
                    opacity: 1;
                }
            }
        }

        .upload-item {
            margin-bottom: 4px;

            &:last-child {
                margin-bottom: 0;
            }

            .upload-item-name {
                font-size: 11px;
                color: var(--text-color);
                white-space: nowrap;
                overflow: hidden;
                text-overflow: ellipsis;
                margin-bottom: 2px;
            }
        }
    }
}

.file-info-drawer {
    .file-info {
        padding: 16px;
        height: 100%;
        box-sizing: border-box;

        .file-info-icon {
            text-align: center;
            margin-bottom: 8px;

            i {
                font-size: 44px;
            }
        }

        .file-info-name {
            text-align: center;
            font-size: 15px;
            font-weight: bold;
            color: var(--text-color);
            word-break: break-all;
        }

        .file-info-path {
            text-align: center;
            font-size: 11px;
            opacity: 0.55;
            word-break: break-all;
            margin: 6px 0 14px;
        }

        .file-info-row {
            display: flex;
            justify-content: space-between;
            gap: 10px;
            padding: 7px 0;
            border-bottom: 1px dashed var(--input-border);
            font-size: 12px;

            .label {
                color: var(--text-color);
                opacity: 0.6;
                flex-shrink: 0;
            }

            .value {
                color: var(--text-color);
                text-align: right;
                word-break: break-all;
            }
        }
    }
}

.uploadContainer {
    .el-upload {
        display: flex;
    }
    .el-upload-dragger {
        width: 95%;
    }
}
.context-menu {
    position: fixed;
    z-index: 3000;
    background: var(--card-bg);
    border: 1px solid var(--input-border);
    border-radius: 4px;
    box-shadow: 0 2px 12px 0 rgba(0,0,0,.1);
    padding: 5px 0;
    min-width: 140px;
}
.context-menu-item {
    padding: 8px 16px;
    cursor: pointer;
    font-size: 13px;
    color: var(--text-color);
    display: flex;
    align-items: center;
    gap: 8px;
    &:hover {
        background: var(--hover-bg, rgba(0, 0, 0, 0.05));
    }
    i {
        font-size: 14px;
    }
}
.context-menu-item.context-danger {
    color: #f56c6c;
}
</style>
