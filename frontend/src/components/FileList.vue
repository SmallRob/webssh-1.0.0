<template>
    <div class="file-list-wrapper">
        <div class="sftp-title">SFTP{{ $t('fileManagement') }}</div>
        <div class="file-header">
            <el-input class="path-input" v-model="currentPath" size="small" @keyup.enter.native="getFileList()" @blur="getFileList" :placeholder="$t('currentPath')"></el-input>
            <el-button-group>
                <el-button type="primary" size="small" icon="el-icon-s-home" @click="goToHome()" :title="$t('home')"></el-button>
                <el-button type="primary" size="small" icon="el-icon-arrow-up" @click="upDirectory()" :title="$t('upDirectory')"></el-button>
                <el-button type="primary" size="small" icon="el-icon-refresh" @click="getFileList()" :title="$t('refresh')"></el-button>
                <el-dropdown @click="openUploadDialog()" @command="handleUploadCommand" size="small">
                    <el-button type="primary" size="small" icon="el-icon-upload"></el-button>
                    <el-dropdown-menu slot="dropdown">
                        <el-dropdown-item command="file">{{ $t('uploadFile') }}</el-dropdown-item>
                        <el-dropdown-item command="folder">{{ $t('uploadFolder') }}</el-dropdown-item>
                    </el-dropdown-menu>
                </el-dropdown>
                <el-button type="primary" size="small" icon="el-icon-folder-add" @click="showNewFolderDialog" :title="$t('newFolder')"></el-button>
                <el-button type="primary" size="small" icon="el-icon-search" @click="showSearchPanel = !showSearchPanel" :title="$t('search')"></el-button>
            </el-button-group>
        </div>

        <el-dialog custom-class="uploadContainer" :title="$t(this.titleTip)" :visible.sync="uploadVisible" append-to-body width="32%">
            <el-upload ref="upload" multiple drag :action="uploadUrl" :data="uploadData" :before-upload="beforeUpload" :on-progress="uploadProgress" :on-success="uploadSuccess">
                <i class="el-icon-upload"></i>
                <div class="el-upload__text">{{ $t(this.selectTip) }}</div>
                <div class="el-upload__tip" slot="tip">{{ this.uploadTip }}</div>
            </el-upload>
        </el-dialog>

        <el-dialog :title="$t('newFolder')" :visible.sync="newFolderVisible" append-to-body width="30%">
            <el-input v-model="newFolderName" :placeholder="$t('enterFolderName')"></el-input>
            <span slot="footer" class="dialog-footer">
                <el-button @click="newFolderVisible = false">{{ $t('Cancel') }}</el-button>
                <el-button type="primary" @click="createNewFolder">{{ $t('OK') }}</el-button>
            </span>
        </el-dialog>

        <el-dialog :title="$t('rename')" :visible.sync="renameDialogVisible" append-to-body width="30%">
            <el-input v-model="newName" :placeholder="$t('enterNewName')"></el-input>
            <span slot="footer" class="dialog-footer">
                <el-button @click="renameDialogVisible = false">{{ $t('Cancel') }}</el-button>
                <el-button type="primary" @click="confirmRename">{{ $t('OK') }}</el-button>
            </span>
        </el-dialog>

        <div v-if="showSearchPanel" class="search-panel">
            <FileSearch @navigate="handleSearchNavigate" @open-file="handleSearchOpenFile" />
        </div>

        <el-table :data="fileList" class="file-table" @row-click="rowClick" @row-contextmenu="showContextMenu" height="100%">
            <el-table-column
                :label="$t('Name')"
                width="140"
                sortable :sort-method="nameSort">
                <template slot-scope="scope">
                    <p v-if="scope.row.IsDir === true" style="color:#0c60b5;cursor:pointer;" class="el-icon-folder"> {{ scope.row.Name }}</p>
                    <p v-else-if="scope.row.IsDir === false" style="cursor: pointer" class="el-icon-document"> {{ scope.row.Name }}</p>
                </template>
            </el-table-column>
            <el-table-column :label="$t('Size')" prop="Size" width="80"></el-table-column>
            <el-table-column :label="$t('ModifiedTime')" prop="ModifyTime" width="120" sortable></el-table-column>
            <el-table-column :label="$t('Actions')" width="60" align="center">
                <template slot-scope="scope">
                    <el-dropdown trigger="click" @command="(cmd) => handleFileCommand(cmd, scope.row)">
                        <el-button size="mini" type="text" icon="el-icon-more"></el-button>
                        <el-dropdown-menu slot="dropdown">
                            <el-dropdown-item command="edit" v-if="!scope.row.IsDir">
                                <i class="el-icon-edit"></i> {{ $t('edit') }}
                            </el-dropdown-item>
                            <el-dropdown-item command="download" v-if="!scope.row.IsDir">
                                <i class="el-icon-download"></i> {{ $t('download') }}
                            </el-dropdown-item>
                            <el-dropdown-item command="rename">
                                <i class="el-icon-edit-outline"></i> {{ $t('rename') }}
                            </el-dropdown-item>
                            <el-dropdown-item command="delete" divided>
                                <i class="el-icon-delete" style="color: #f56c6c;"></i> <span style="color: #f56c6c;">{{ $t('delete') }}</span>
                            </el-dropdown-item>
                        </el-dropdown-menu>
                    </el-dropdown>
                </template>
            </el-table-column>
        </el-table>

        <div ref="contextMenu" v-show="contextMenuVisible" class="context-menu" :style="{ left: contextMenuX + 'px', top: contextMenuY + 'px' }">
            <div class="context-menu-item" @click="handleContextCommand('edit')" v-if="!contextMenuRow.IsDir">
                <i class="el-icon-edit"></i> {{ $t('edit') }}
            </div>
            <div class="context-menu-item" @click="handleContextCommand('download')" v-if="!contextMenuRow.IsDir">
                <i class="el-icon-download"></i> {{ $t('download') }}
            </div>
            <div class="context-menu-item" @click="handleContextCommand('rename')">
                <i class="el-icon-edit-outline"></i> {{ $t('rename') }}
            </div>
            <div class="context-menu-item" @click="handleContextCommand('delete')" style="color: #f56c6c;">
                <i class="el-icon-delete"></i> {{ $t('delete') }}
            </div>
        </div>

        <FileEditor v-model="editorVisible" :filePath="editingFilePath" @saved="getFileList" />
    </div>
</template>

<script>
import { fileList, deleteFile, renameFile, createFolder } from '@/api/file'
import { mapState } from 'vuex'
import FileEditor from './FileEditor.vue'
import FileSearch from './FileSearch.vue'

export default {
    name: 'FileList',
    components: { FileEditor, FileSearch },
    data() {
        return {
            uploadVisible: false,
            fileList: [],
            downloadFilePath: '',
            currentPath: '/',
            selectTip: 'clickSelectFile',
            titleTip: 'uploadFile',
            uploadTip: '',
            progressPercent: 0,
            initialRedirectDone: false,
            homePath: '',
            showSearchPanel: false,
            newFolderVisible: false,
            newFolderName: '',
            renameDialogVisible: false,
            newName: '',
            renamingRow: null,
            contextMenuVisible: false,
            contextMenuX: 0,
            contextMenuY: 0,
            contextMenuRow: {},
            editorVisible: false,
            editingFilePath: ''
        }
    },
    mounted() {
        if (!this.currentPath || this.currentPath === '/') {
            this.getFileList()
        }
        document.addEventListener('click', this.hideContextMenu)
    },
    beforeDestroy() {
        document.removeEventListener('click', this.hideContextMenu)
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
            this.currentPath = this.currentTab && this.currentTab.path ? this.currentTab.path : '/';
        }
    },
    methods: {
        goToHome() {
            if (this.homePath) {
                if (this.currentPath !== this.homePath) {
                    this.currentPath = this.homePath;
                    this.getFileList();
                }
            } else {
                this.$message.warning(this.$t('homeNotReady'));
            }
        },
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
                ws.onclose = () => {}
                ws.onerror = () => {}
            }
        },
        nameSort(a, b) {
            return a.Name > b.Name
        },
        rowClick(row) {
            if (row.IsDir) {
                this.currentPath = this.currentPath.charAt(this.currentPath.length - 1) === '/' ? this.currentPath + row.Name : this.currentPath + '/' + row.Name
                this.getFileList()
            } else {
                this.downloadFilePath = this.currentPath.charAt(this.currentPath.length - 1) === '/' ? this.currentPath + row.Name : this.currentPath + '/' + row.Name
                this.downloadFile()
            }
        },
        async getFileList() {
            this.currentPath = this.currentPath.replace(/\\+/g, '/')
            if (this.currentPath === '') {
                this.currentPath = '/'
            }
            this.$store.commit('SET_CURRENT_PATH', this.currentPath)
            const result = await fileList(this.currentPath, this.$store.getters.sshReq)
            if (result.Msg === 'success') {
                if (result.Data.home) {
                    this.homePath = result.Data.home;
                }
                if (result.Data.list === null) {
                    this.fileList = []
                } else {
                    this.fileList = result.Data.list
                }
                if (!this.initialRedirectDone && result.Data.home && result.Data.home !== '/' && this.currentPath !== result.Data.home) {
                    this.initialRedirectDone = true
                    this.currentPath = result.Data.home
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
        },
        upDirectory() {
            if (this.currentPath === '/') {
                return
            }
            let pathList = this.currentPath.split('/')
            if (pathList[pathList.length - 1] === '') {
                pathList = pathList.slice(0, pathList.length - 2)
            } else {
                pathList = pathList.slice(0, pathList.length - 1)
            }
            this.currentPath = pathList.length === 1 ? '/' : pathList.join('/')
            this.getFileList()
        },
        downloadFile() {
            const prefix = process.env.NODE_ENV === 'production' ? `${location.origin}` : 'api'
            const downloadUrl = `${prefix}/file/download?path=${this.downloadFilePath}&sshInfo=${this.$store.getters.sshReq}`
            window.open(downloadUrl)
        },
        showNewFolderDialog() {
            this.newFolderName = ''
            this.newFolderVisible = true
        },
        async createNewFolder() {
            if (!this.newFolderName.trim()) {
                this.$message.warning(this.$t('enterFolderName'))
                return
            }
            const path = this.currentPath.endsWith('/') ? this.currentPath + this.newFolderName : this.currentPath + '/' + this.newFolderName
            try {
                const result = await createFolder(path, this.$store.getters.sshReq)
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
        showContextMenu(row, column, event) {
            event.preventDefault()
            this.contextMenuRow = row
            this.contextMenuX = event.clientX
            this.contextMenuY = event.clientY
            this.contextMenuVisible = true
        },
        hideContextMenu() {
            this.contextMenuVisible = false
        },
        handleContextCommand(cmd) {
            this.handleFileCommand(cmd, this.contextMenuRow)
            this.hideContextMenu()
        },
        handleFileCommand(cmd, row) {
            const path = this.currentPath.endsWith('/') ? this.currentPath + row.Name : this.currentPath + '/' + row.Name
            switch (cmd) {
                case 'edit':
                    this.editingFilePath = path
                    this.editorVisible = true
                    break
                case 'download':
                    this.downloadFilePath = path
                    this.downloadFile()
                    break
                case 'rename':
                    this.renamingRow = row
                    this.newName = row.Name
                    this.renameDialogVisible = true
                    break
                case 'delete':
                    this.confirmDelete(path, row.Name)
                    break
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
        async confirmRename() {
            if (!this.newName.trim()) {
                this.$message.warning(this.$t('enterNewName'))
                return
            }
            const oldPath = this.currentPath.endsWith('/') ? this.currentPath + this.renamingRow.Name : this.currentPath + '/' + this.renamingRow.Name
            const newPath = this.currentPath.endsWith('/') ? this.currentPath + this.newName : this.currentPath + '/' + this.newName
            try {
                const result = await renameFile(oldPath, newPath, this.$store.getters.sshReq)
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
        handleSearchNavigate(path) {
            this.currentPath = path
            this.showSearchPanel = false
            this.getFileList()
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

    .sftp-title {
        font-size: 16px;
        font-weight: bold;
        color: var(--text-color);
        text-align: center;
        padding-bottom: 8px;
        margin-bottom: 8px;
        border-bottom: 1px solid var(--input-border);
        flex-shrink: 0;
    }

    .file-header {
        flex-shrink: 0;
        margin-bottom: 10px;
        display: flex;
        align-items: center;
    }

    .path-input {
        flex: 1;
        padding: 0 5px;
        margin-right: 2px;
    }

    .file-header .el-button-group .el-button {
        padding: 8px;
        width: 36px;
        height: 32px;
        line-height: 1;
    }

    .search-panel {
        flex-shrink: 0;
        max-height: 300px;
        overflow: hidden;
        border: 1px solid var(--input-border);
        border-radius: 4px;
        margin-bottom: 10px;
    }

    .file-table {
        flex-grow: 1;
        width: 100%;
        & .el-table__body-wrapper {
            height: calc(100% - 40px) !important;
        }
        &.el-table th {
            height: 40px;
            padding: 0;
        }
        &.el-table td {
            padding: 0;
        }
        .cell {
            padding: 1px 0px 2px 5px;
            line-height: 1.1;
            p {
              margin: 0;
            }
        }
        th > .cell {
            display: flex;
            align-items: center;
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
    min-width: 120px;
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
</style>
