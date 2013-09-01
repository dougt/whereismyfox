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

    $("#devices").html(table);
    $("#devices button").click(function() {
        var select = $(this).parent().prev().children("select"),
        options = select.children("option"),
        idx = select.get(0).selectedIndex,
        trigger = $(options[idx]).attr("data-trigger");

        $.post(trigger);
    });
}

/*
 * Takes an array of jQuery XHR requests, and returns a Promise that resolves
 * when all of then succeed and rejects when any of them fails. The Promise
 * resolves to an array, each element being the result of each request in the
 * order they were passed.
 *
 * NOTE: jQuery.when() when called with just one promise (e.g. in the case
 * where the user has only one device registered), passes the result of that
 * promise to the associated callback, while for multiple devices, it passes
 * a list.  This is stupid behaviour for our case.
 *
 */
function URLs_every(deferreds) {
    return $.when.apply(null, deferreds).then(function() {
        if (deferreds.length == 1) {
            return [arguments[0][0]];
        } else {
            return Array.prototype.slice.call(arguments).map(function(jqXHRResult) { return jqXHRResult[0]; });
        }
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

        URLs_every(deviceRequests).then(function(devices) {
            var commandRequests = devices.map(function(device) {
                return $.getJSON('/device/' + device.Id + '/command');
            });

            URLs_every(commandRequests).then(function(commands) {
                for (var i = 0; i < devices.length; i++) {
                    devices[i].commands = commands[i];
                    devices[i].id = i;
                }
                console.log(devices);
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

