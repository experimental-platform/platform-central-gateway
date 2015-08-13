# Experimental Platform: Central Gateway

This Microservice acts as simple router for the Experimental Platform.


This is a component of the experimental platform. To read more about it please go here:

[https://github.com/experimental-platform/platform-configure-script](https://github.com/experimental-platform/platform-configure-script)

## Description

On requests it inspects the request and decide where the request should be forwarded.
The following illustration show both possible forward destinations.

```
                ┌───────────┐
                │►►Request◄◄│
                └───────────┘
                      │
                      │
                      ▼
        ┌───────────────────────────┐
        │                           │
        │      Central-Gateway      │
        │                           │
        └───────────────────────────┘
                      ││
          ┌───────────┘└──────────┐
          │                       │
          ▼                       ▼
┌───────────────────┐   ┌───────────────────┐
│ Platform-Frontend │   │       Dokku       │
│ (Admin Interface) │   │      (Apps)       │
└───────────────────┘   └───────────────────┘
```



## Branch: Development

[![Circle CI](https://circleci.com/gh/experimental-platform/platform-central-gateway.svg?style=svg&circle-token=d8d407dd0b16973dd5bcc10e474db66e9036ce65)](https://circleci.com/gh/experimental-platform/platform-central-gateway)

All development branches stem from and (re-)integrate here.

## Branch: Master

[![Circle CI](https://circleci.com/gh/experimental-platform/platform-central-gateway/tree/master.svg?style=svg&circle-token=d8d407dd0b16973dd5bcc10e474db66e9036ce65)](https://circleci.com/gh/experimental-platform/platform-central-gateway/tree/master)

This is the base for &alpha;-channel releases.