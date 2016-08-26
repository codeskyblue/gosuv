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

var ws;
var wsProtocol = location.protocol == "https:" ? "wss" : "ws";

var vm = new Vue({
    el: "#app",
    data: {
        programs: [{
            program: {
                name: "gggg",
                command: "",
                dir: "",
                autoStart: true,
            },
            status: "running",
        }],
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

    console.log("HEE")
    ws = new WebSocket(wsProtocol + "://" + location.host + "/ws/events");
    ws.onopen = function(evt) {
        console.log("OPEN");
    }
    ws.onclose = function(evt) {
        console.log("CLOSE");
        ws = null;
    }
    ws.onmessage = function(evt) {
        console.log("response:" + evt.data);
        vm.refresh();
    }
    ws.onerror = function(evt) {
        console.log("error:", evt.data);
    }

    // ws.send("Hello")

});
