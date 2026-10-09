<template>
  <!-- 传输队列浮窗：全局共享队列（Vuex），上传/下载均在此展示实时进度 -->
  <div class="transfer-panel" v-show="visible && items.length > 0">
    <div class="tp-header">
      <i class="el-icon-sort"></i>
      <span class="tp-title">{{ $t('transferQueue') }}</span>
      <span class="tp-summary">{{ transferActive }}/{{ items.length }}</span>
      <i class="el-icon-close tp-close" @click="close"></i>
    </div>
    <el-table :data="items" size="mini" max-height="320" class="tp-table" :empty-text="$t('transferEmpty')">
      <el-table-column :label="$t('fileName')" min-width="130">
        <template slot-scope="{ row }">
          <span class="tp-name" :title="row.name">{{ row.name }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('colAction')" width="62" align="center">
        <template slot-scope="{ row }">
          <span class="tp-op" :class="row.action === 'upload' ? 'tp-op-up' : 'tp-op-down'">
            {{ row.action === 'upload' ? $t('opUpload') : $t('opDownload') }}
          </span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('fileSize')" width="78">
        <template slot-scope="{ row }">{{ fmtSize(row.size) }}</template>
      </el-table-column>
      <el-table-column :label="$t('colSource')" min-width="100">
        <template slot-scope="{ row }">
          <span class="tp-path" :title="row.source">{{ row.source }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('colTarget')" min-width="110">
        <template slot-scope="{ row }">
          <span class="tp-path" :title="row.target">{{ row.target }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('colProgress')" width="150">
        <template slot-scope="{ row }">
          <el-progress
            :percentage="normalizePercent(row.percent)"
            :stroke-width="6"
            :status="row.status === 'error' ? 'exception' : (isDone(row) ? 'success' : undefined)"
            :format="p => p + '%'"></el-progress>
          <div class="tp-status" :class="'tp-st-' + row.status">{{ statusText(row) }}</div>
        </template>
      </el-table-column>
      <el-table-column :label="$t('colFailReason')" min-width="90">
        <template slot-scope="{ row }">
          <span class="tp-error" :title="row.error">{{ row.error || '-' }}</span>
        </template>
      </el-table-column>
    </el-table>
    <div class="tp-footer">
      <el-button size="mini" @click="clearList">{{ $t('clearList') }}</el-button>
      <el-button size="mini" type="primary" @click="close">{{ $t('btnClose') }}</el-button>
    </div>
  </div>
</template>

<script>
import { mapState } from 'vuex'

export default {
    name: 'TransferPanel',
    computed: {
        ...mapState({
            items: state => state.transfer.items,
            visible: state => state.transfer.visible
        }),
        transferActive() {
            return this.items.filter(i => i.status === 'uploading' || i.status === 'downloading').length
        }
    },
    methods: {
        close() {
            this.$store.commit('TRANSFER_VISIBLE', false)
        },
        clearList() {
            this.$store.commit('TRANSFER_CLEAR')
        },
        isDone(row) {
            return row.status === 'success'
        },
        normalizePercent(p) {
            return Math.max(0, Math.min(100, Math.floor(p || 0)))
        },
        statusText(row) {
            if (row.status === 'error') return this.$t('transferFailed')
            if (row.status === 'success') return this.$t('transferDone')
            if (row.status === 'downloading') return this.$t('transferDownloading')
            return this.$t('transferUploading')
        },
        fmtSize(bytes) {
            if (!bytes || bytes <= 0) return '-'
            const units = ['B', 'KB', 'MB', 'GB', 'TB']
            let v = bytes, i = 0
            while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
            return (v >= 100 ? v.toFixed(0) : v.toFixed(1)) + units[i]
        }
    }
}
</script>

<style scoped>
.transfer-panel {
  position: fixed;
  right: 16px;
  bottom: 16px;
  width: 760px;
  max-width: calc(100vw - 32px);
  background: var(--card-bg, #fff);
  border: 1px solid var(--input-border, #dcdfe6);
  border-radius: 8px;
  box-shadow: 0 8px 28px rgba(0, 0, 0, 0.22);
  z-index: 3000;
  padding: 10px 12px 8px 12px;
}

.tp-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, #303133);
  padding-bottom: 8px;
}

.tp-summary {
  font-size: 12px;
  font-weight: normal;
  color: var(--text-secondary, #909399);
}

.tp-close {
  margin-left: auto;
  cursor: pointer;
  font-size: 15px;
  color: var(--text-secondary, #909399);
}

.tp-close:hover {
  color: var(--text-primary, #303133);
}

.tp-table {
  width: 100%;
}

.tp-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
}

.tp-path {
  font-size: 12px;
  color: var(--text-secondary, #909399);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
}

.tp-op {
  font-size: 12px;
  padding: 1px 6px;
  border-radius: 3px;
}

.tp-op-up {
  color: #67c23a;
  background: rgba(103, 194, 58, 0.12);
}

.tp-op-down {
  color: #409eff;
  background: rgba(64, 158, 255, 0.12);
}

.tp-status {
  font-size: 11px;
  margin-top: 1px;
}

.tp-st-success { color: #67c23a; }
.tp-st-error { color: #f56c6c; }
.tp-st-uploading,
.tp-st-downloading { color: var(--text-secondary, #909399); }

.tp-error {
  font-size: 12px;
  color: #f56c6c;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
}

.tp-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 8px;
}
</style>
