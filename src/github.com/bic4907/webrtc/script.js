// get DOM elements
dataChannelLog = document.getElementById('data-channel'),
iceConnectionLog = document.getElementById('ice-connection-state')
iceGatheringLog = document.getElementById('ice-gathering-state')
signalingLog = document.getElementById('signaling-state')
latencyLog = document.getElementById('latency')

document.getElementById('video').muted = true






// peer connection
let pc = null;
// data channel
let dc = null, dcInterval = null;
let pingTable = {}

function createPeerConnection() {

    let config = {
        sdpSemantics: 'unified-plan',
        iceServers: [{urls: ['stun:stun.l.google.com:19302']}]
    };
    pc = new RTCPeerConnection(config);

    pc.addEventListener('icegatheringstatechange', function() {
        iceGatheringLog.textContent += ' -> ' + pc.iceGatheringState;
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

    navigator.mediaDevices.getUserMedia({video: true, audio: true})
        .then(stream => {
            pc.addStream(document.getElementById('video').srcObject = stream)
            pc.createOffer().then(d => pc.setLocalDescription(d))
        })

    pc.oniceconnectionstatechange = event => {
        console.log(pc.iceConnectionState)
    }

    pc.onicecandidate = event => {
        if (event.candidate === null) {
            $.ajax({
                url: '/connect',
                method: 'POST',
                async: false,
                data: {
                    localDescription: btoa(JSON.stringify(pc.localDescription))
                },
            }).success(function(data) {
                pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(data))))
            })

        }
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