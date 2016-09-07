/* Javascript */
function pathJoin(parts, sep) {
    var separator = sep || '/';
    var replace = new RegExp(separator + '{1,}', 'g');
    return parts.join(separator).replace(replace, separator);
}

function getQueryString(name) {
    var reg = new RegExp("(^|&)" + name + "=([^&]*)(&|$)");
    var r = decodeURI(window.location.search).substr(1).match(reg);
    if (r != null) return r[2].replace(/\+/g, ' ');
    return null;
}


function newWebsocket(pathname, opts) {
    var wsProtocol = location.protocol == "https:" ? "wss" : "ws";
    var ws = new WebSocket(wsProtocol + "://" + location.host + pathname);
    opts = opts || {};
    ws.onopen = opts.onopen || function(evt) {
        console.log("WS OPEN", pathname);
    }
    ws.onclose = opts.onclose || function(evt) {
        console.log("CLOSE");
        ws = null;
    }
    ws.onmessage = opts.onmessage || function(evt) {
        console.log("response:" + evt.data);
    }
    ws.onerror = function(evt) {
        console.error("error:", evt.data);
    }
    return ws;
}

function formatBytes(value) {
    var bytes = parseFloat(value);
    if (bytes < 0) return "-";
    else if (bytes < 1024) return bytes + " B";
    else if (bytes < 1048576) return (bytes / 1024).toFixed(0) + " KB";
    else if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + " MB";
    else return (bytes / 1073741824).toFixed(1) + " GB";
}
