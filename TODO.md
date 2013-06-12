App
===

Only do setup once in the app
-----------------------------

We don't want to re-do the setup every time we open the app. It could
be interesting to just redirect whereismyfox.com when the app is opened
after the first-time setup.

Note, however, that due to a bug in push, we currentl only get notifications
when the app is in its index.html page, so using anything else as a landing
page might break the app entirely.

Unregistration
--------------

Need to think about this. Do we want to allow unregistering from the app, after
login? Do we want to unregister when the app is uninstalled?

Icons
-----

Would be cool to replace the current icons with actual WhereIsMyFox icons ;)

Server
======

Work on post-registration screen
--------------------------------

Right now we display a very minimal response from the server when the app
completes its setup. We could work on this response page to make it look better,
but bearing in mind the note above about the bug in push, it could also make
sense to just XHR the information to the server (instead of using a form) and
ignoring its response entirely, so we stay in index.html.

whereismyfox.com
================

Fix dumb polling
-----------------

Right now we just periodically refresh the whole list of devices when the user
clicks "Where Is It?" for any device, and keep refreshing even if there are no
more devices.

We could be smarter about this by only refreshing the lost device, and detecting
when there are no more devices for example. A more sophisticated solution could
be to detect push support in the browser, and register a channel per device so
the server can notify the front-end of updates.

Better Google Maps integration
------------------------------

Instead of just linking to the Google Maps query for the coordinates, we could
use the Maps API to actually display a map with the device's location (and
perhaps track it).

Stop tracking
-------------

When do we stop tracking? A button in the site could trigger another push
notification to the app so it stops tracking.
