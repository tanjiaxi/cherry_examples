/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-25 00:29:56
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-25 15:27:46
 * @FilePath: /examples/demo_cluster/nodes/web/static/pb-extract.js
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
// pb.js 加载后提取 proto 对象
// 这个文件需要在 pb.js 之后加载

(function(global) {
    'use strict';
    
    console.log('Extracting proto objects from module.exports...');
    console.log('module:', typeof module);
    console.log('exports:', typeof exports);
    
    // 检查 module.exports 的内容
    if (typeof module !== 'undefined' && module.exports) {
        console.log('module.exports keys:', Object.keys(module.exports));
        console.log('module.exports.proto:', typeof module.exports.proto);
        console.log('module.exports.pb:', typeof module.exports.pb);
        
        // 尝试多种方式提取 proto 对象
        if (module.exports.proto && module.exports.proto.pb) {
            // 方式1: module.exports.proto.pb
            global.proto = module.exports.proto;
            console.log('✅ Method 1: Extracted proto from module.exports.proto');
        } else if (module.exports.pb) {
            // 方式2: module.exports.pb
            global.proto = { pb: module.exports.pb };
            console.log('✅ Method 2: Extracted proto.pb from module.exports.pb');
        } else {
            // 方式3: 直接使用 module.exports 作为 proto.pb
            // 检查是否有 protobuf 消息的特征（如 serializeBinary 方法）
            var hasProtoMessages = false;
            for (var key in module.exports) {
                if (module.exports[key] && typeof module.exports[key] === 'function') {
                    hasProtoMessages = true;
                    break;
                }
            }
            
            if (hasProtoMessages) {
                global.proto = { pb: module.exports };
                console.log('✅ Method 3: Using module.exports as proto.pb');
            } else {
                console.error('❌ No proto messages found in module.exports');
                // 尝试从 exports 对象获取
                if (typeof exports !== 'undefined' && exports !== module.exports) {
                    console.log('Trying exports object...');
                    console.log('exports keys:', Object.keys(exports));
                    global.proto = { pb: exports };
                    console.log('✅ Method 4: Using exports as proto.pb');
                }
            }
        }
        
        // 显示提取到的内容
        if (global.proto && global.proto.pb) {
            var pbKeys = Object.keys(global.proto.pb);
            console.log('proto.pb keys (' + pbKeys.length + '):', pbKeys.slice(0, 10).join(', ') + (pbKeys.length > 10 ? '...' : ''));
        }
    } else {
        console.error('❌ module or module.exports not found');
    }
    
    // 验证关键对象是否存在
    if (typeof proto !== 'undefined' && proto.pb) {
        var requiredMessages = [
            'LoginRequest', 'LoginResponse',
            'EnterMachine', 'EnterMachineResponse', 
            'MachineInfo', 'MachineInfoResponse',
            'Spin', 'SpinResponse'
        ];
        var found = [];
        var missing = [];
        
        requiredMessages.forEach(function(msgName) {
            if (typeof proto.pb[msgName] !== 'undefined') {
                found.push(msgName);
            } else {
                missing.push(msgName);
            }
        });
        
        if (found.length > 0) {
            console.log('✅ Found messages:', found.join(', '));
        }
        if (missing.length > 0) {
            console.warn('⚠️  Missing messages:', missing.join(', '));
        }
    } else {
        console.error('❌ proto.pb not available');
    }
    
})(this);
