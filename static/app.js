var personaArguments = {
    siteName: 'Where Is My Fox?',
};

function renderDeviceTable(devices) {
    if (devices.length) {
        devices[0].first = true;
    }

    var table = Mustache.render($('#device-list-template').html(), {
        devices: devices,
        mapsURL: "https://maps.google.com/maps?q="
    });
    console.log(table);

    $("#devices").html(table);
    $("#devices button").click(function() {
        var select = $(this).parent().prev().children("select"),
        options = select.children("option"),
        idx = select.get(0).selectedIndex,
        trigger = $(options[idx]).attr("data-trigger");

        $.post(trigger);
    });
}

function updateDevices() {
    $("#devices").html("Fetching list...");

    function failedToFetchDevices() {
        $("#devices").html("Failed to fetch your devices!");
    }

    $.getJSON('/device').then(function(list) {
        var deviceRequests = list.map(function(url) {
            return $.getJSON(url);
        });

        $.when.apply(null, deviceRequests).then(function() {
            var devices = [];

            if (deviceRequests.length == 1) {
                devices.push(arguments[0]);
            } else {
                for (var i = 0; i < arguments.length; i++) {
                    devices.push(arguments[i][0]);
                }
            }

            var commandRequests = devices.map(function(device) {
                return $.getJSON('/device/' + device.Id + '/command');
            });

            $.when.apply(null, commandRequests).then(function() {
                for (var i = 0; i < devices.length; i++) {
                    devices[i].commands = arguments[i];
                }

                renderDeviceTable(devices);
            });

        }, failedToFetchDevices);
    }, failedToFetchDevices);
}

$("document").ready(function(){

    $("#persona-logout").hide();
    $("#devices").hide();

    function loggedIn(email){
        $("#persona-login").hide();
        $("#persona-logout").show();

        $("#devices").show();
        updateDevices();
    }

    function loggedOut(){
        $("#persona-logout").hide();
        $("#persona-login").show();
        $("#devices").hide();
    }

    $("#persona-login").on("click", function(e) {
        e.preventDefault();
        navigator.id.get(mailVerified, personaArguments);
    });

    $("#persona-logout").on("click", function(e) {
        e.preventDefault();
        $.get('/auth/logout', loggedOut);
    });

    function mailVerified(assertion){
        $.ajax({
            type: 'POST',
            url: '/auth/login',
            data: {assertion: assertion},
            success: function(res, status, xhr) {
                loggedIn(JSON.parse(res).email);
            },
            error: function(xhr, status, err) {
                alert("Login failure: " + err);
            }
        });
    }

    $.get('/auth/check', function (res) {
        if (res === "") {
            loggedOut();
        } else {
            loggedIn(res);
        }
    });
});

