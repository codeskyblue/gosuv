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

var refreshPrograms = function() {
    $.ajax({
        url: "/api/programs",
        success: function(data) {
            console.log(data)
            vm.programs = data;
        }
    });
}

$(function() {
    refreshPrograms();

    $("#formNewProgram").submit(function(e) {
        var url = "/api/programs",
            data = $(this).serialize();
        $.ajax({
            type: "POST",
            url: url,
            data: data,
            success: function(data) {
                console.log(data);
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

});
