/* javascript */
var vm = new Vue({
    el: '#app',
    data: {
        name: name,
        pid: '-',
        subPids: [],
    }
});

var ws = newWebsocket('/ws/perfs/' + name, {
    onopen: function(evt) {
        console.log(evt);
    },
    onmessage: function(evt) {
        var data = JSON.parse(evt.data);
        vm.pid = data.pid;
        vm.subPids = data.sub_pids;
        console.log("pid", data.pid, data); //evt.data.pid);
        if (memData && data.mem && data.mem.Resident) {
            memData.push({
                value: [new Date(), data.mem.Resident],
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
