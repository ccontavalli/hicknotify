# hicknotify

This repository contains a small [go](http://golang.org) program to:

   * Monitor a set of HIKVISION Cameras.
   * Send notifications via email if any of the cameras report any event, or connectivity to the camera is lost.
   * Typical events are the PIR sensors detecting movement, camera covered, disconnected, ... any event the
     software on the camera can detect.

hicknotify will, however:

   * Try to avoid sending multiple notifications for the same event.
     Eg, if the camera reports the same event multiple times within an interval, only one notification
     is sent.
   * Try hard to connect to your camera. Retry if it fails / connection is dropped.
     Notify you if the camera becomes unreachable for longer than a configured time.
   * Monitor an additional set of IP addresses in your network, and disable notifications
     if any of those IPs are reachable. This is handy, for example, to disable
     notifications based on the presence of your phone in the house connected
     to wifi, or your wifi connected car in the garage.

The software was written based on specifications from HIKVISION, which you can find
[here](http://oversea-download.hikvision.com/uploadfile/Leaflet/ISAPI/HIKVISION%20ISAPI_2.0-IPMD%20Service.pdf).

If the link is out of date, you can probably find them by searching for "HIKVISION" "ISAPI" "alarmStream" or
similar on your favourite search engine.

## Why did you write this?

A few reasons:

   1. It was my starting point for an home automation project using HIKVISION APIs. Maybe it can be useful to you as well.
   2. HIKVISION Cameras and NVRs can send email notifications directly, without use of `hicknotify`. However, it is harder to integrate with
      other automation (example: don't notify if you can detect my phone at home).
   3. ... additionally, using any form of notification from the Cameras or NVR
      directly requires them having direct internet connectivity.  And when
      talking about cameras watching me at home, running proprietary software
      I have no control over, I start having serious concerns related to privacy.

      So, they live in a dedicated network that provides them no internet connectivity.
      On that network, I have only one little box I own, running a variation
      on `hicknotify` that monitors the camera and is able to send notifications.

## How do I use this?

Install the tool:

    go get github.com/ccontavalli/hicknotify

Create a `config.json` file in the same directory:

    {
      # List of cameras to monitor, the URL that generates the 'alertStream'.
      # Note that if you have a NVR, you still need to provide the URL of the
      # alertStream of each camera independently.
      # Monitoring the NVR itself may provide some notifications like diskfull,
      # error, ... but not the cameras' sensors.
      # 
      # If your cameras are plugged into the NVR, and not accessible directly,
      # many HIKVISION NVRs have a feature called 'Virtual Host' that will
      # instruct the NVR to export the cameras directly on the same IP under
      # different ports. You should find the option under
      # "Config -> Network- > Advanced -> Other Tab"
      "Cameras": [
        {"Url": "http://nvr:65010/ISAPI/Event/notification/alertStream", "Name": "Living Room"},
        {"Url": "http://nvr:65011/ISAPI/Event/notification/alertStream", "Name": "Garage"},
        {"Url": "http://nvr:65012/ISAPI/Event/notification/alertStream", "Name": "Kitchen"},
        {"Url": "http://nvr:65013/ISAPI/Event/notification/alertStream", "Name": "Stairs"}
      ],
    
      # Username to use to access the camera.
      # You probably want to create a dedicated user and password
      # with only access to alerts notifications.
      "Username": "username",
      "Password": "password",
    
      # Address 'From' which notifications are generated.
      "MailFrom": "cameras@blahblahblah.net",
      # Who to send the notification to.
      "MailTo": [
          "you@youremail.com"
      ],
      # Email server to use to send emails out.
      "MailServer": "smtp.gmail.com",
      "MailPort": 587,
      "MailUser": "yourgmailuser@gmail.com",
      "MailPassword": "yourgmailpassword",
    
      # If any of those hosts are pingable, than don't
      # send notifications. I use the IP address associated
      # with my phone, so when I'm home, no notifications
      # are sent.
      "Hosts": [
        "10.10.10.240",
        "10.10.10.241"
      ],
      # How often to ping the IP addresses above, in seconds.
      "PingInterval": 1,
      # If any of the IPs above is pingable, how long to disable
      # notifications for.
      "PingDisable": 600

      # When an event is observed, don't send additional notifications
      # unless the event goes away for at least 'DampeningTime' seconds.
      "DampeningTime": 120,
      # Send a notification if a camera disappears for longer than
      # 'WatchdogTime'seconds.
      "WatchdogTime": 10,
      # If a connection is dropped, a camera becomes unreachable, ...
      # don't retry to connect more often than once per 'ErrorRetryTime'.
      "ErrorRetryTime": 5
    }

Run `./hicknotify` from the same directory where you stored your `config.json`.
Look for errors in the output.
