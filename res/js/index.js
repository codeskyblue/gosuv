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

var wsProtocol = location.protocol == "https:" ? "wss" : "ws";
var W = {};

var testPrograms = [{
    program: {
        name: "gggg",
        command: "",
        dir: "",
        autoStart: true,
    },
    status: "running",
}];

var vm = new Vue({
    el: "#app",
    data: {
        isConnectionAlive: true,
        log: {
            content: '',
            follow: true,
            line_count: 0,
        },
        programs: [],
    },
    methods: {
        addNewProgram: function() {
            console.log("Add")
            var form = $("#formNewProgram");
            form.submit(function(e) {
                console.log("HellO")
                e.preventDefault();
                console.log(e);
                $("#newProgram").modal('hide')
                return false;
            });
            // console.log(this.program.name);
        },
        updateBreadcrumb: function() {
            var pathname = decodeURI(location.pathname || "/");
            var parts = pathname.split('/');
            this.breadcrumb = [];
            if (pathname == "/") {
                return this.breadcrumb;
            }
            var i = 2;
            for (; i <= parts.length; i += 1) {
                var name = parts[i - 1];
                var path = parts.slice(0, i).join('/');
                this.breadcrumb.push({
                    name: name + (i == parts.length ? ' /' : ''),
                    path: path
                })
            }
            return this.breadcrumb;
        },
        refresh: function() {
            // ws.send("Hello")
            console.log("RR");
            $.ajax({
                url: "/api/programs",
                success: function(data) {
                    vm.programs = data;
                }
            });
        },
        test: function() {
            ws.send("Test");
        },
        cmdStart: function(name) {
            console.log(name);
            $.ajax({
                url: "/api/programs/" + name + "/start",
                method: 'post',
                success: function(data) {
                    console.log(data);
                }
            })
        },
        cmdStop: function(name) {
            $.ajax({
                url: "/api/programs/" + name + "/stop",
                method: 'post',
                success: function(data) {
                    console.log(data);
                }
            })
        },
        canStop: function(status) {
            switch (status) {
                case "running":
                case "retry wait":
                    return true;
            }
        }
    }
})

Vue.filter('fromNow', function(value) {
    return moment(value).fromNow();
})

Vue.filter('formatBytes', function(value) {
    var bytes = parseFloat(value);
    if (bytes < 0) return "-";
    else if (bytes < 1024) return bytes + " B";
    else if (bytes < 1048576) return (bytes / 1024).toFixed(0) + " KB";
    else if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + " MB";
    else return (bytes / 1073741824).toFixed(1) + " GB";
})

Vue.directive('disable', function(value) {
    this.el.disabled = !!value
})

$(function() {
    vm.refresh();

    $("#formNewProgram").submit(function(e) {
        var url = "/api/programs",
            data = $(this).serialize();
        $.ajax({
            type: "POST",
            url: url,
            data: data,
            success: function(data) {
                if (data.status === 0) {
                    $("#newProgram").modal('hide');
                } else {
                    window.alert(data.error);
                }
            },
            error: function(err) {
                console.log(err.responseText);
            }
        })
        e.preventDefault()
    });


    function newWebsocket(pathname, opts) {
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

    console.log("HEE")

    function newEventWatcher() {
        W.events = newWebsocket("/ws/events", {
            onopen: function(evt) {
                vm.isConnectionAlive = true;
            },
            onmessage: function(evt) {
                console.log("response:" + evt.data);
                vm.refresh();
            },
            onclose: function(evt) {
                W.events = null;
                vm.isConnectionAlive = false;
                console.log("Reconnect after 3s")
                setTimeout(newEventWatcher, 3000)
            }
        });
    };

    newEventWatcher();

    // cancel follow log if people want to see the original data
    $(".realtime-log").bind('mousewheel', function(evt) {
        if (evt.originalEvent.wheelDelta >= 0) {
            vm.log.follow = false;
        }
    })

    var ws = newWebsocket("/ws/logs/hee", {
        onopen: function(evt) {
            vm.log.content = "";
        },
        onmessage: function(evt) {
            vm.log.content += evt.data;
            vm.log.line_count = $.trim(vm.log.content).split(/\r\n|\r|\n/).length;
            if (vm.log.follow) {
                var pre = $(".realtime-log")[0];
                pre.scrollTop = pre.scrollHeight;
            }
        }
    })

});
