/* index.js */
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
    edit: {
      program: null,
    }
  },
  methods: {
    addNewProgram: function() {
      console.log("Add")
      var form = $("#formNewProgram");
      form.submit(function(e) {
        e.preventDefault();
        $("#newProgram").modal('hide')
        return false;
      });
    },
    showEditProgram: function(p) {
      this.edit.program = Object.assign({}, p); // here require polyfill.min.js
      $("#programEdit").modal('show');
    },
    editProgram: function() {
      var p = this.edit.program;
      $.ajax({
          url: "/api/programs/" + p.name,
          method: "PUT",
          data: JSON.stringify(p),
        })
        .then(function(ret) {
          console.log(ret);
          $("#programEdit").modal('hide');
        })
        // console.log(JSON.stringify(p));
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
      $.ajax({
        url: "/api/programs",
        success: function(data) {
          vm.programs = data;
          Vue.nextTick(function() {
            $('[data-toggle="tooltip"]').tooltip()
          })
        }
      });
    },
    reload: function() {
      $.ajax({
        url: "/api/reload",
        method: "POST",
        success: function(data) {
          if (data.status == 0) {
            alert("reload success");
          } else {
            alert(data.value);
          }
        }
      });
    },
    test: function() {
      console.log("test");
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
    cmdTail: function(name) {
      var that = this;
      if (W.wsLog) {
        W.wsLog.close()
      }
      W.wsLog = newWebsocket("/ws/logs/" + name, {
        onopen: function(evt) {
          that.log.content = "";
          that.log.line_count = 0;
        },
        onmessage: function(evt) {
          // strip ansi color
          // console.log("DT:", evt.data)
          that.log.content += evt.data.replace(/\033\[[0-9;]*m/g, "");
          that.log.line_count = $.trim(that.log.content).split(/\r\n|\r|\n/).length;
          if (that.log.follow) {
            var pre = $(".realtime-log")[0];
            setTimeout(function() {
              pre.scrollTop = pre.scrollHeight - pre.clientHeight;
            }, 1);
          }
        }
      });
      this.log.follow = true;
      $("#modalTailf").modal({
        show: true,
        keyboard: true,
        // keyboard: false,
        // backdrop: 'static',
      })
    },
    cmdDelete: function(name) {
      if (!confirm("Confirm delete \"" + name + "\"")) {
        return
      }
      $.ajax({
        url: "/api/programs/" + name,
        method: 'delete',
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

Vue.filter('colorStatus', function(value) {
  var makeColorText = function(text, color) {
    return "<span class='status' style='background-color:" + color + "'>" + text + "</span>";
  }
  switch (value) {
    case "stopping":
      return makeColorText(value, "#996633");
    case "running":
      return makeColorText(value, "green");
    case "fatal":
      return makeColorText(value, "red");
    default:
      return makeColorText(value, "gray");
  }
})

Vue.directive('disable', function(value) {
  this.el.disabled = !!value
})

$(function() {
  vm.refresh();

  $("#formNewProgram").submit(function(e) {
    var url = "/api/programs",
      data = $(this).serialize(),
      name = $(this).find("[name=name]").val(),
      disablechars = "./\\";

    if (!name) {
        alert("\"" + name + "\" is empty ")
        return false
    }
    if (disablechars.indexOf(name[0]) != -1) {
        alert("\"" + name + "\" Can't starts with \".\" \"/\" \"\\\"")
        return false
    }
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
  $('#modalTailf').on('hidden.bs.modal', function() {
    // do something…
    console.log("Hiddeen")
    if (W.wsLog) {
      console.log("wsLog closed")
      W.wsLog.close()
    }
  })
});