<template>
  <div>
    <!-- 预制命令列表：点击即发送到当前 SSH 终端执行 -->
    <el-dialog :title="$t('presetCommands')" :visible.sync="show" append-to-body width="520px" custom-class="preset-dialog">
      <div class="preset-list" v-loading="loading">
        <div v-if="!loading && presets.length === 0" class="preset-empty">
          <i class="el-icon-collection-tag"></i>
          <div>{{ $t('presetEmpty') }}</div>
          <div class="preset-empty-tip" v-if="isAdmin">{{ $t('presetEmptyTip') }}</div>
        </div>
        <div class="preset-item" v-for="p in presets" :key="p.id" @click="run(p)">
          <div class="preset-item-main">
            <span class="preset-name">{{ p.name }}</span>
            <span class="preset-cmd" :title="p.command">{{ p.command }}</span>
          </div>
          <i class="el-icon-video-play preset-run-icon" :title="$t('runCommand')"></i>
        </div>
      </div>
      <span slot="footer" class="preset-footer">
        <el-button v-if="isAdmin" size="small" icon="el-icon-setting" @click="openConfig">{{ $t('presetConfig') }}</el-button>
        <el-button size="small" @click="show = false">{{ $t('btnClose') }}</el-button>
      </span>
    </el-dialog>

    <!-- 管理员配置对话框：整表编辑，保存即全局生效 -->
    <el-dialog :title="$t('presetConfig')" :visible.sync="configVisible" append-to-body width="640px" custom-class="preset-config-dialog">
      <el-table :data="draft" size="mini" max-height="360" :empty-text="$t('presetEmpty')">
        <el-table-column :label="$t('presetName')" width="170">
          <template slot-scope="{ row }">
            <el-input v-model="row.name" size="mini" :placeholder="$t('presetName')" maxlength="50"></el-input>
          </template>
        </el-table-column>
        <el-table-column :label="$t('presetCommandContent')">
          <template slot-scope="{ row }">
            <el-input v-model="row.command" size="mini" :placeholder="$t('presetCommandContent')"></el-input>
          </template>
        </el-table-column>
        <el-table-column width="46" align="center">
          <template slot-scope="{ $index }">
            <i class="el-icon-delete preset-del" @click="draft.splice($index, 1)"></i>
          </template>
        </el-table-column>
      </el-table>
      <div class="preset-add-row">
        <el-button size="mini" type="text" icon="el-icon-plus" @click="addRow">{{ $t('presetAdd') }}</el-button>
        <span class="preset-tip">{{ $t('presetConfigTip') }}</span>
      </div>
      <span slot="footer" class="preset-footer">
        <el-button size="small" @click="configVisible = false">{{ $t('Cancel') }}</el-button>
        <el-button size="small" type="primary" :loading="saving" @click="save">{{ $t('OK') }}</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import { listPresets, savePresets, getAdminStatus } from '@/api/common'

export default {
    name: 'PresetCommands',
    data() {
        return {
            show: false,
            loading: false,
            presets: [],
            isAdmin: false,
            adminChecked: false,
            configVisible: false,
            draft: [],
            saving: false
        }
    },
    watch: {
        show(val) {
            if (val) {
                this.fetchPresets()
                this.fetchAdminStatus()
            }
        }
    },
    methods: {
        open() {
            this.show = true
        },
        async fetchPresets() {
            this.loading = true
            try {
                const res = await listPresets()
                if (res.Msg === 'success' && res.Data) {
                    this.presets = res.Data.list || []
                } else {
                    this.$message.error(res.Msg || this.$t('presetLoadFail'))
                }
            } catch (e) {
                this.$message.error(this.$t('presetLoadFail'))
            } finally {
                this.loading = false
            }
        },
        async fetchAdminStatus() {
            if (this.adminChecked) return
            try {
                const res = await getAdminStatus()
                if (res.Msg === 'success' && res.Data) {
                    this.isAdmin = !!res.Data.isAdmin
                    this.adminChecked = true
                }
            } catch (e) { /* 门禁未启用时后端也返回 isAdmin，失败则按非管理员处理 */ }
        },
        run(p) {
            this.$emit('run', p.command)
            this.show = false
        },
        openConfig() {
            this.draft = this.presets.map(p => ({ id: p.id, name: p.name, command: p.command }))
            if (this.draft.length === 0) {
                this.addRow()
            }
            this.configVisible = true
        },
        addRow() {
            this.draft.push({ id: '', name: '', command: '' })
        },
        async save() {
            const rows = this.draft.filter(r => r.name.trim() || r.command.trim())
            for (const r of rows) {
                if (!r.name.trim() || !r.command.trim()) {
                    this.$message.warning(this.$t('presetNameAndCmdRequired'))
                    return
                }
            }
            this.saving = true
            try {
                const res = await savePresets(rows)
                if (res.Msg === 'success') {
                    this.$message.success(this.$t('presetSaved'))
                    this.configVisible = false
                    this.presets = (res.Data && res.Data.list) || rows
                } else {
                    this.$message.error(res.Msg)
                }
            } catch (e) {
                this.$message.error(this.$t('presetSaveFail'))
            } finally {
                this.saving = false
            }
        }
    }
}
</script>

<style scoped>
.preset-list {
  max-height: 380px;
  overflow-y: auto;
}

.preset-empty {
  text-align: center;
  padding: 32px 0;
  color: var(--text-secondary, #909399);
}

.preset-empty i {
  font-size: 34px;
  margin-bottom: 8px;
}

.preset-empty-tip {
  font-size: 12px;
  margin-top: 6px;
  opacity: 0.7;
}

.preset-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 12px;
  border-radius: 6px;
  cursor: pointer;
  border: 1px solid transparent;
}

.preset-item:hover {
  background: rgba(64, 158, 255, 0.08);
  border-color: rgba(64, 158, 255, 0.25);
}

.preset-item + .preset-item {
  margin-top: 4px;
}

.preset-item-main {
  flex: 1;
  min-width: 0;
}

.preset-name {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary, #303133);
}

.preset-cmd {
  display: block;
  font-size: 12px;
  color: var(--text-secondary, #909399);
  font-family: 'DejaVu Sans Mono', Consolas, monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-top: 2px;
}

.preset-run-icon {
  font-size: 18px;
  color: #409eff;
  opacity: 0;
  transition: opacity 0.15s;
}

.preset-item:hover .preset-run-icon {
  opacity: 1;
}

.preset-footer {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.preset-del {
  font-size: 15px;
  color: #f56c6c;
  cursor: pointer;
  margin-top: 6px;
}

.preset-add-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 4px;
}

.preset-tip {
  font-size: 12px;
  color: var(--text-secondary, #909399);
}
</style>
