<template>
    <div class="file-search-wrapper">
        <div class="search-header">
            <el-input
                v-model="keyword"
                :placeholder="$t('searchPlaceholder')"
                size="small"
                clearable
                @keyup.enter.native="doSearch"
                class="search-input">
                <el-button slot="append" icon="el-icon-search" @click="doSearch" :loading="searching"></el-button>
            </el-input>
        </div>
        <div v-if="searchResults.length > 0" class="search-results">
            <div class="results-header">
                <span>{{ $t('searchResults') }} ({{ searchResults.length }})</span>
                <el-button size="mini" type="text" @click="clearResults">{{ $t('clear') }}</el-button>
            </div>
            <div class="results-list">
                <div
                    v-for="(item, index) in searchResults"
                    :key="index"
                    class="result-item"
                    @click="handleResultClick(item)">
                    <i :class="item.isDir ? 'el-icon-folder' : 'el-icon-document'" :style="{ color: item.isDir ? '#0c60b5' : '' }"></i>
                    <div class="result-info">
                        <div class="result-name">{{ item.name }}</div>
                        <div class="result-path">{{ item.path }}</div>
                    </div>
                    <span class="result-size">{{ item.size }}</span>
                </div>
            </div>
        </div>
        <div v-else-if="searched && !searching" class="no-results">
            <i class="el-icon-search"></i>
            <span>{{ $t('noResults') }}</span>
        </div>
    </div>
</template>

<script>
import { searchFiles } from '@/api/file'

export default {
    name: 'FileSearch',
    data() {
        return {
            keyword: '',
            searchResults: [],
            searching: false,
            searched: false
        }
    },
    methods: {
        async doSearch() {
            if (!this.keyword.trim()) {
                this.$message.warning(this.$t('enterKeyword'))
                return
            }
            this.searching = true
            this.searched = true
            try {
                const sshInfo = this.$store.getters.sshReq
                const currentPath = this.$store.state.currentPath || '/'
                const result = await searchFiles(this.keyword, currentPath, sshInfo)
                if (result.Msg === 'success') {
                    this.searchResults = result.Data.results || []
                } else {
                    this.$message.error(result.Msg)
                    this.searchResults = []
                }
            } catch (e) {
                this.$message.error(this.$t('searchError'))
                this.searchResults = []
            } finally {
                this.searching = false
            }
        },
        handleResultClick(item) {
            if (item.isDir) {
                this.$emit('navigate', item.path)
            } else {
                this.$emit('open-file', item.path)
            }
        },
        clearResults() {
            this.searchResults = []
            this.keyword = ''
            this.searched = false
        }
    }
}
</script>

<style lang="scss">
.file-search-wrapper {
    padding: 10px;
    display: flex;
    flex-direction: column;
    height: 100%;
}
.search-header {
    flex-shrink: 0;
    margin-bottom: 10px;
}
.search-results {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
}
.results-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 5px 0;
    font-size: 12px;
    color: var(--text-color);
    border-bottom: 1px solid var(--input-border);
    margin-bottom: 5px;
}
.results-list {
    flex: 1;
    overflow-y: auto;
}
.result-item {
    display: flex;
    align-items: center;
    padding: 8px 5px;
    cursor: pointer;
    border-radius: 4px;
    gap: 8px;
    &:hover {
        background: var(--hover-bg, rgba(0, 0, 0, 0.05));
    }
    i {
        font-size: 16px;
        flex-shrink: 0;
    }
    .result-info {
        flex: 1;
        min-width: 0;
        .result-name {
            font-size: 13px;
            color: var(--text-color);
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }
        .result-path {
            font-size: 11px;
            color: #999;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }
    }
    .result-size {
        font-size: 11px;
        color: #999;
        flex-shrink: 0;
    }
}
.no-results {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100px;
    color: #999;
    i {
        font-size: 24px;
        margin-bottom: 10px;
    }
}
</style>
