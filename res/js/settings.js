/* javascript */
var vm = new Vue({
    el: '#app',
    data: {
        name: name,
        pid: '-',
    }
});

var ws = newWebsocket('/ws/perfs/' + name, {
    onopen: function(evt) {
        console.log(evt);
    },
    onmessage: function(evt) {
        var data = JSON.parse(evt.data);
        vm.pid = data.pid;
        console.log("pid", data.pid, data); //evt.data.pid);
        if (memData && data.mem && data.mem.rss) {
            memData.push({
                value: [new Date(), data.mem.rss],
            })
            if (memData.length > 10) {
                memData.shift();
            }
            chartMem.setOption({
                series: [{
                    data: memData,
                }]
            });
        }
        if (cpuData && data.cpu !== undefined) {
            cpuData.push({
                value: [new Date(), data.cpu],
            })
            if (cpuData.length > 10) {
                cpuData.shift();
            }
            chartCpu.setOption({
                series: [{
                    data: cpuData,
                }]
            })
        }
    }
})
