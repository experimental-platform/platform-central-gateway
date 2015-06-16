# Central Gateway

This Microservice acts as simple router for the Experimental Platform.

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
