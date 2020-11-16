
$.urlParam = function(name){ var results = new RegExp('[\?&]' + name + '=([^&#]*)').exec(window.location.href); if (results==null){ return null; } else{ return results[1] || 0; } }

let app = new Vue({
    el: '#app',
    data: {
        pc: null,
        ws: null,

        mode: null,
        userId: null,
        roomId: null,

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
        remoteStream: new MediaStream(),

        latency: null,
        status: 'disconnected',

    },
    mounted: function () {

        let param = $.urlParam('mode');
        param == 'sender' ? this.mode = 'sender' : this.mode = 'receiver'

        let userId = $.urlParam('userId');
        userId == null ? this.userId = 'TestUser' : this.userId = userId

        let roomId = $.urlParam('roomId');
        roomId == null ? this.roomId = 'TestRoom' : this.roomId = roomId



        this.initialize()
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
    watch: {
      mode: function(newVal, oldVal) {
        this.initialize()
      },
    },
    methods: {
        initialize:  function() {
            this.addLog('info', this.mode)

            if(this.mode == 'sender') {
                this.showWebcamVideo()
            }


            if(this.mode == 'receiver') {
                this.remoteStream = new MediaStream()
                document.getElementById("video").srcObject = this.remoteStream
            }
        },
        showWebcamVideo: function () {
            let self = this
            navigator.mediaDevices.getUserMedia(self.resourceType).then(function (stream) {
                document.getElementById('video').srcObject = stream
                document.getElementById('video').muted = true
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
                        try {
                            self.pc.addIceCandidate(candidate).then(evt => {
                                clearInterval(itv)
                            })
                        } catch {}
                    }, 1000)
                } else if(json.type === "duplicatedSession") {
                    self.addLog('error', 'Duplicated session')
                    self.disconnect()
                } else if(json.type === "remoteOffer") {
                    self.pc.setRemoteDescription(new RTCSessionDescription(json.message)).then(() => {
                        self.pc.createAnswer().then(answer => {
                            self.pc.setLocalDescription(answer)

                            self.ws.send(JSON.stringify({
                                type: 'remoteAnswer',
                                message: btoa(JSON.stringify(self.pc.currentLocalDescription)),
                            }))
                        })
                        self.addLog('debug', 'Broadcaster connected')
                    })
                } else if(json.type === "broadcasterExited") {
                    self.pc.setRemoteDescription(new RTCSessionDescription(json.message)).then(() => {
                        self.pc.createAnswer().then(answer => {
                            self.pc.setLocalDescription(answer)

                            self.ws.send(JSON.stringify({
                                type: 'remoteAnswer',
                                message: btoa(JSON.stringify(self.pc.currentLocalDescription)),
                            }))

                            self.addLog('debug', 'Broadcaster disconnected')
                        })
                    })



                }
            }
        },
        disconnect: function () {

            if(this.ws) this.ws.close()
            if(this.pc) this.pc.close()

            this.ws = null
            this.pc = null

            this.status = 'disconnected'
            this.latency = null

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
                    for (let track of stream.getTracks()) {
                        self.pc.addTrack(track);
                    }

                    self.pc.createOffer().then(d => {

                        self.pc.setLocalDescription(d)

                        this.ws.send(JSON.stringify({
                            type: 'broadcastRequest',
                            message: btoa(JSON.stringify(self.pc.localDescription)),
                            userId: self.userId,
                            roomId: self.roomId,
                        }))


                    })
                })
        },
        initalizeReceiver: function (track) {
            let self = this

            self.pc = new RTCPeerConnection(self.pcSetting);
            self.attachPeerConnectionHandler()
            self.initializeHealthCheck()

            self.remoteStream = new MediaStream()
            document.getElementById("video").srcObject = self.remoteStream

            self.pc.addEventListener('track', function(event) {
                self.status = 'connected'
                console.log('track', event.track)

                self.remoteStream.addTrack(event.track, self.remoteStream)
            });


            self.pc.createOffer().then(d => {

                self.pc.setLocalDescription(d).then(() => {

                    self.ws.send(JSON.stringify({
                        type: 'subscribeRequest',
                        message: btoa(JSON.stringify(self.pc.localDescription)),
                        userId: self.userId,
                        roomId: self.roomId,
                    }))

                })


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