/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-25 00:29:45
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-25 00:30:14
 * @FilePath: /examples/demo_cluster/nodes/web/static/pb-wrapper.js
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
// pb.js 浏览器兼容包装器
// 这个文件需要在 pb.js 之前加载

(function(global) {
    'use strict';
    
    console.log('Initializing pb-wrapper...');
    
    // 检查 jspb 是否已加载
    if (typeof jspb === 'undefined') {
        console.error('❌ jspb (google-protobuf) not loaded! Please load it before pb-wrapper.js');
    } else {
        console.log('✅ jspb (google-protobuf) is available');
    }
    
    // 创建模拟的 CommonJS 环境
    var module = { exports: {} };
    var exports = module.exports;
    
    // 模拟 require 函数
    global.require = function(name) {
        console.log('require() called with:', name);
        if (name === 'google-protobuf') {
            // 返回 jspb 对象
            if (typeof jspb !== 'undefined') {
                return jspb;
            }
            console.error('google-protobuf (jspb) not loaded');
            return {};
        }
        // 对于其他模块，返回空对象
        return {};
    };
    
    // 保存 module 和 exports 到全局
    global.module = module;
    global.exports = exports;
    
    // 确保 goog 对象存在（protobuf 生成的代码可能需要）
    if (typeof goog === 'undefined') {
        global.goog = jspb;
        console.log('✅ Created goog object from jspb');
    }
    
    console.log('✅ pb-wrapper initialized successfully');
    console.log('  - module:', typeof module);
    console.log('  - exports:', typeof exports);
    console.log('  - require:', typeof require);
    console.log('  - jspb:', typeof jspb);
    console.log('  - goog:', typeof goog);
    
})(this);
