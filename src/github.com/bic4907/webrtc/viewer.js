// get DOM elements
dataChannelLog = document.getElementById('data-channel')
iceConnectionLog = document.getElementById('ice-connection-state')
iceGatheringLog = document.getElementById('ice-gathering-state')
signalingLog = document.getElementById('signaling-state')
latencyLog = document.getElementById('latency')






let remoteStream
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

    pc.addEventListener('track', function(event) {
        console.log(event);

        setStatus('connected')
        remoteStream.addTrack(event.track, remoteStream)

        document.getElementById("video").srcObject = remoteStream

    });



    pc.addEventListener('icecandidate', function(e) {
        if(e == null || e.candidate == null) return

        $.ajax({
            url: '/viewer-add-candidate',
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
                    url: '/viewer-get-candidate',
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
                        //console.log(candidate)

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



    pc.oniceconnectionstatechange = event => {
        console.log(pc.iceConnectionState)
    }


    dc = pc.createDataChannel('health-check')
    dc2 = pc.createDataChannel('track')

    dc.addEventListener('open', event => {
        let count = 1000
        dcInterval = setInterval(function() {
            dc.send('ping-' + count)
            pingTable['ping-' + count] = (new Date).getMilliseconds()
            count += 1000
        }, 500)

    })


    dc.addEventListener('message', event => {
        if(event.data == 'video-ok') {
            setStatus('connected')
        }
        if(event.data.toString().startsWith('pong')) {
            arr = event.data.toString().split('-')

            prev = pingTable['ping-' + arr[1]]
            gap = (new Date).getMilliseconds() - prev
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

    remoteStream = new MediaStream();

    pc = createPeerConnection();
    pc.addTransceiver('video')
    pc.addTransceiver('audio')

    pc.createOffer().then(d => {

        pc.setLocalDescription(d)

        $.ajax({
            url: '/viewer-connect',
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



    })



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
