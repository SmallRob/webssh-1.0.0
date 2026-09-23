const path = require('path');
const proxyTarget = 'http://172.188.122.74:19888';
const wsTarget = proxyTarget.replace('http', 'ws');

module.exports = {
  publicPath: '/',
  outputDir: '../public',
  assetsDir: 'static',
  lintOnSave: true,
  productionSourceMap: false,
  parallel: require('os').cpus().length > 1,

  devServer: {
    disableHostCheck: false,
    open: process.platform === 'darwin',
    host: '0.0.0.0',
    port: 8257,
    https: false,
    hotOnly: false,
    open: true,
    proxy: {
      '/api': {
        target: proxyTarget,
        changeOrigin: true,
        ws: true,
        pathRewrite: {
          '^/api': ''
        }
      },
      '/ws': {
        target: wsTarget,
        changeOrigin: true,
        ws: true,
        pathRewrite: {
          '^/ws': ''
        }
      }
    }
  },

  css: {
    loaderOptions: {
      sass: {
        sassOptions: {
          silenceDeprecations: ['legacy-js-api'],
        },
      },
    },
  },

  configureWebpack: (config) => {
    config.performance = {
      hints: false
    };

    // 生产环境优化配置
    if (process.env.NODE_ENV === 'production') {
      config.optimization = {
        splitChunks: {
          chunks: 'all',
          minSize: 20000,
          maxSize: 512000,
          maxAsyncRequests: 30,
          maxInitialRequests: 30,
          automaticNameDelimiter: '-',
          cacheGroups: {
            vendors: {
              name: 'chunk-vendors',
              test: /[\\/]node_modules[\\/]/,
              priority: -10,
              chunks: 'initial'
            },
            elementUI: {
              name: 'chunk-elementUI',
              priority: 20,
              test: /[\\/]node_modules[\\/]element-ui[\\/]/
            },
            common: {
              name: 'chunk-common',
              minChunks: 2,
              priority: -20,
              chunks: 'initial',
              reuseExistingChunk: true
            }
          }
        }
      };

      // Gzip 压缩
      try {
        const CompressionPlugin = require('compression-webpack-plugin');
        config.plugins.push(
          new CompressionPlugin({
            algorithm: 'gzip',
            test: /\.(js|css|html|svg)$/,
            threshold: 10240,
            minRatio: 0.8
          })
        );
      } catch (e) {
        console.warn('compression-webpack-plugin 未安装，跳过 Gzip 压缩');
      }
    }
  },

  chainWebpack: (config) => {
    config.plugins.delete('prefetch');

    // 拷贝 public 下的静态资源到 ../public（Go 通过 go:embed public/* 内嵌）
    // - img：原有背景图
    // - novnc：noVNC 1.7.0（原生 ESM，浏览器直接 import，不经 webpack）
    // - rdp：ironrdp-wasm（WASM 版 RDP 协议栈 + .wasm 二进制）
    //   RDP/VNC 客户端刻意走「原生 ESM 静态资源」而非打包，原因是二者
    //   依赖 import.meta.url 解析 .wasm 位置，且体积/生命周期与业务代码无关。
    config.plugin('copy').tap(() => {
      return [
        [
          {
            from: path.resolve(__dirname, 'public/img'),
            to: path.resolve(__dirname, '../public/static/img')
          },
          {
            from: path.resolve(__dirname, 'public/novnc'),
            to: path.resolve(__dirname, '../public/static/novnc')
          },
          {
            from: path.resolve(__dirname, 'public/rdp'),
            to: path.resolve(__dirname, '../public/static/rdp')
          }
        ]
      ];
    });

    // 生产环境 source map
    if (process.env.NODE_ENV === 'production') {
      config.devtool('source-map');
    }
  }
};