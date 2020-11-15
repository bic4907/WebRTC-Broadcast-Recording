
$.urlParam = function(name){ var results = new RegExp('[\?&]' + name + '=([^&#]*)').exec(window.location.href); if (results==null){ return null; } else{ return results[1] || 0; } }

let app = new Vue({
    el: '#app',
    data: {
        pc: null,
        ws: null,

        mode: null,

        resourceType: {
            audio: true,
            video: true
        },
        pcSetting: {
            sdpSemantics: 'unified-plan',
            iceServers: [
                {urls: ['stun:stun.l.google.com:19302']},
            ],
            iceRestart: true,
        },

        pingTable: {},

        logs: [],
        videoViewer: null,

        latency: null,
        status: 'disconnected'
    },
    mounted: function () {

        let param = $.urlParam('mode');
        param == 'sender' ? this.mode = 'sender' : this.mode = 'receiver'

        this.addLog('info', this.mode)

        if(this.mode == 'sender') {
            this.showWebcamVideo()
        }
    },
    computed: {
        wsUrl: function() {
            let url
            if (location.protocol === "https:") {
                url = 'wss://' + document.location.host + '/ws'
            } else {
                url = 'ws://' + document.location.host + '/ws'
            }
            return url
        }
    },
    methods: {
        showWebcamVideo: function () {
            let self = this
            navigator.mediaDevices.getUserMedia(self.resourceType).then(function (stream) {
                document.getElementById('video').srcObject = stream
                self.addLog('info', '리소스가져오는데 성공함')
            }, function (err) {
                self.addLog('error', '리소스를 가져올 수 없음')
            });
        },
        connect: function () {
            let self = this

            self.status = 'connecting'

            if (window["WebSocket"] == null) {
                this.addLog('error', 'Browser doesn\'t support websocket')
                return
            }

            this.addLog('info', 'Connecting websocket...')
            this.mode == 'sender' ? this.connectAsSender() : this.connectAsReceiver()

            this.ws.onmessage = function (evt) {

                let json = JSON.parse(evt.data)

                if(json.type === "remoteDescription") {
                    self.pc.setRemoteDescription(new RTCSessionDescription(json.message))
                } else if(json.type === "iceCandidate") {
                    if(json.message == null) return

                    let candidate = new RTCIceCandidate(JSON.parse(json.message))
                    let itv = setInterval(function() {
                        self.pc.addIceCandidate(candidate).then(evt => {
                            clearInterval(itv)

                        })
                    }, 1000)
                }
            }
        },
        disconnect: function () {

            if(this.status == 'connecting') return

            this.ws.close()
            this.pc.close()

            this.ws = null
            this.pc = null

            this.status = 'disconnected'
            this.latency = null

            if(this.mode == 'receiver') {
                document.getElementById("video").srcObject = null
            }

        },
        connectAsSender: function () {
            let self = this

            this.ws = new WebSocket(self.wsUrl);
            this.ws.onclose = function (evt) {
                self.disconnect()
                self.addLog('debug', '웹소켓 접속종료')
            };
            this.ws.onopen = function (evt) {
                self.addLog('info', '웹소켓 접속 완료')
                self.initalizeSender()
            }
        },
        connectAsReceiver: function () {
            let self = this

            this.ws = new WebSocket(self.wsUrl);
            this.ws.onclose = function (evt) {
                self.disconnect()
                self.addLog('debug', '웹소켓 접속종료')
            };
            this.ws.onopen = function (evt) {
                self.addLog('info', '웹소켓 접속 완료')
                self.initalizeReceiver()
            }
        },
        attachPeerConnectionHandler: function() {
            let self = this

            this.pc.addEventListener('iceconnectionstatechange', function() {
                 self.addLog('debug', 'ICEConnectionState changed to ' + self.pc.iceConnectionState)
            }, false);


            this.pc.addEventListener('signalingstatechange', function() {
                self.addLog('debug', 'SignalingState changed to ' + self.pc.signalingState)
            }, false);

            this.pc.addEventListener('icegatheringstatechange', function() {
                self.addLog('debug', 'ICEGatheringState changed to ' + self.pc.iceGatheringState)
            }, false);


            this.pc.addEventListener('icecandidate', function(e) {
                if(e.candidate == null) return

                self.ws.send(
                    JSON.stringify({
                        type: 'iceCandidate',
                        message: JSON.stringify(e.candidate),
                    })
                )
            })
        },
        initalizeSender: function () {
            let self = this

            self.pc = new RTCPeerConnection(self.pcSetting);
            self.attachPeerConnectionHandler()
            self.initializeHealthCheck()
            self.initializeVideoCheck()


            navigator.mediaDevices.getUserMedia({video: true, audio: true})
                .then(stream => {
                    for (const track of stream.getTracks()) {
                        self.pc.addTrack(track);
                    }

                    self.pc.createOffer().then(d => {

                        self.pc.setLocalDescription(d)

                        let user_id = 'user1'
                        let room_id = 'r1'

                        this.ws.send(JSON.stringify({
                            type: 'broadcastRequest',
                            message: btoa(JSON.stringify(self.pc.localDescription)),
                            user_id: user_id,
                            room_id: room_id,
                        }))


                    })
                })
        },
        initalizeReceiver: function (track) {

            let self = this

            self.pc = new RTCPeerConnection(self.pcSetting);
            self.pc.addTransceiver('video')
            self.pc.addTransceiver('audio')
            self.attachPeerConnectionHandler()
            self.initializeHealthCheck()


            let remoteStream = new MediaStream();
            self.pc.addEventListener('track', function(event) {
                self.status = 'connected'
                remoteStream.addTrack(event.track, remoteStream)
                document.getElementById("video").srcObject = remoteStream
            });


            self.pc.createOffer().then(d => {

                self.pc.setLocalDescription(d)

                let user_id = 'user1'
                let room_id = 'r1'

                this.ws.send(JSON.stringify({
                    type: 'subscribeRequest',
                    message: btoa(JSON.stringify(self.pc.localDescription)),
                    user_id: user_id,
                    room_id: room_id,
                }))
            })
        },
        initializeHealthCheck: function() {
            let self = this

            self.pingTable = {}

            let dc = self.pc.createDataChannel('health-check')
            dc.addEventListener('open', event => {
                let count = 1000
                let itv = setInterval(function() {
                    try {
                        dc.send('ping-' + count)
                        self.pingTable['ping-' + count] = (new Date).getMilliseconds()
                        count += 1000
                    } catch {
                        clearInterval(itv)
                    }
                }, 500)
            })
            dc.addEventListener('message', event => {
                if(event.data.toString().startsWith('pong')) {
                    let arr = event.data.toString().split('-')
                    let prev = self.pingTable['ping-' + arr[1]]
                    let gap = (new Date).getMilliseconds() - prev
                    self.latency = gap + "ms"
                }
            })
        },
        initializeVideoCheck: function() {
            let self = this

            let dc = self.pc.createDataChannel('video-check')
            dc.addEventListener('message', event => {
                if(event.data == 'video-ok') {
                    self.status = 'connected'
                }
            })
        },
        addLog: function(type, message) {
            this.logs.push({type: type, message: message})
        },
        clearLog: function() {
            this.logs.clear()
        }
    }
})
/*
$.ajax({
url: '/connect',
method: 'POST',
async: false,
data: {
    localDescription: btoa(JSON.stringify(pc.localDescription))
},
}).success(function(data) {
arr = data.split('\t')
console.log(arr)
uid = arr[0]
desc = JSON.parse(atob(arr[1]))
pc.setRemoteDescription(new RTCSessionDescription(desc))
})




},
initalizeReceiver: function() {


},
addLog: function(type, message) {
this.logs.push({type: type, message: message})
},
clearLog: function() {
this.logs.clear()
}
}


})












/*

dataChannelLog = document.getElementById('data-channel'),
iceConnectionLog = document.getElementById('ice-connection-state')
iceGatheringLog = document.getElementById('ice-gathering-state')
signalingLog = document.getElementById('signaling-state')
latencyLog = document.getElementById('latency')

document.getElementById('video').muted = true






// peer connection
let pc = null;
let uid = null;

// data channel
let dc = null, dcInterval = null;
let candidateItv = null
let pingTable = {}

function createPeerConnection() {

let config = {
sdpSemantics: 'unified-plan',
iceServers: [
{urls: ['stun:stun.l.google.com:19302']},
]
};
pc = new RTCPeerConnection(config);

pc.addEventListener('icegatheringstatechange', function() {
iceGatheringLog.textContent += ' -> ' + pc.iceGatheringState;


console.log(pc.iceGatheringState)



}, false);
iceGatheringLog.textContent = pc.iceGatheringState;

pc.addEventListener('iceconnectionstatechange', function() {
iceConnectionLog.textContent += ' -> ' + pc.iceConnectionState;

}, false);
iceConnectionLog.textContent = pc.iceConnectionState;

pc.addEventListener('signalingstatechange', function() {
signalingLog.textContent += ' -> ' + pc.signalingState;
}, false);
signalingLog.textContent = pc.signalingState



pc.addEventListener('icecandidate', function(e) {
if(e == null || e.candidate == null) return

$.ajax({
url: '/add-candidate',
method: 'POST',
data: {
    uid: uid,
    candidate: JSON.stringify(e.candidate)
},
}).success(function(data) {
arr = data.split('\t')
uid = arr[0]
desc = JSON.parse(atob(arr[1]))
//pc.setRemoteDescription(new RTCSessionDescription(desc))
})

if(candidateItv == null) {
candidateItv = setInterval(function() {
    $.ajax({
        url: '/get-candidate',
        method: 'POST',
        data: {
            uid: uid,
        },
    }).success(function(data) {
        arr = data.split('\t')
        uid = arr[0]
        desc = JSON.parse(atob(arr[1]))
        candidates = JSON.parse(arr[2])

        candidates.forEach(element => {

            candidate = JSON.parse(element)
            console.log(candidate)

            pc.addIceCandidate(candidate)
        });




        //pc.setRemoteDescription(new RTCSessionDescription(desc))
    })

    if(pc.iceConnectionState == 'connected') {
        clearInterval(candidateItv)
    }


}, 300)
}

})


navigator.mediaDevices.getUserMedia({video: true, audio: true})
.then(stream => {
pc.addStream(document.getElementById('video').srcObject = stream)
pc.createOffer().then(d => {

    pc.setLocalDescription(d)

    let user_id = prompt('User ID를 입력하세요', '')

    $.ajax({
        url: '/connect',
        method: 'POST',
        async: false,
        data: {
            localDescription: btoa(JSON.stringify(pc.localDescription)),
            user_id: user_id
        },
    }).success(function(data) {
        arr = data.split('\t')
        console.log(arr)
        uid = arr[0]
        desc = JSON.parse(atob(arr[1]))
        pc.setRemoteDescription(new RTCSessionDescription(desc))
    })



})
})


/*
$.ajax({
url: '/connect',
method: 'POST',
async: false,
data: {
    localDescription: btoa(JSON.stringify(pc.localDescription))
},
}).success(function(data) {
arr = data.split('\t')
console.log(arr)
uid = arr[0]
desc = JSON.parse(atob(arr[1]))
pc.setRemoteDescription(new RTCSessionDescription(desc))
})


*/
/*
    pc.oniceconnectionstatechange = event => {
        console.log(pc.iceConnectionState)
    }


    dc = pc.createDataChannel('health-check')

    dc.addEventListener('open', event => {
        let count = 1000
        dcInterval = setInterval(function() {
            dc.send('ping-' + count)
            pingTable['ping-' + count] = (new Date).getMilliseconds()
            count += 1000
        }, 500)

    })
    dc.addEventListener('message', event => {
        console.log(event.data)
        if(event.data == 'video-ok') {
            setStatus('connected')
        }
        if(event.data.toString().startsWith('pong')) {
            arr = event.data.toString().split('-')

            prev = pingTable['ping-' + arr[1]]
            gap = (new Date).getMilliseconds() - prev
            console.log(gap)
            setLatency(gap)
        } 

    })


    return pc;
}


var constraints = {
    audio: true,
    video: true
};

function start() {
    setStatus('connecting')

    document.getElementById('start').style.display = 'none';

    pc = createPeerConnection();


    document.getElementById('stop').style.display = 'inline-block';
}

function stop() {
    setStatus('disconnected')

    document.getElementById('stop').style.display = 'none';
    document.getElementById('start').style.display = 'inline-block';

    // close data channel
    if (dc) {
        dc.close();
    }

    // close transceivers
    if (pc.getTransceivers) {
        pc.getTransceivers().forEach(function(transceiver) {
            if (transceiver.stop) {
                transceiver.stop();
            }
        });
    }

    // close local audio / video
    pc.getSenders().forEach(function(sender) {
        sender.track.stop();
    });


    if(dcInterval != null) {
        clearInterval(dcInterval)
        dcInterval = null
    }
    if(candidateItv != null) {
        clearInterval(candidateItv)
        candidateItv = null
    }
    

    // close peer connection
    setTimeout(function() {
        pc.close();
    }, 500);
}


navigator.mediaDevices.getUserMedia(constraints).then(function(stream) {

    document.getElementById('video').srcObject = stream;

}, function(err) {
    alert('Could not acquire media: ' + err);
});

function setStatus(value) {
    $('.status .connected').hide()
    $('.status .connecting').hide()
    $('.status .disconnected').hide()
    $('.status .' + value).show()
}
setStatus('disconnected')

function setLatency(value) {
    latencyLog.innerText = " (" + value.toString() + "ms" + ")"
}
*/