# Product Vision

## Problem

Surf forecast products expose wave, swell, wind, and tide data, but a surfer still has to translate those values into a decision for a particular spot, skill level, board, preference, and available time window.

## Vision

The Search will be a self-hostable surf intelligence application that turns attributable forecast data and explicit local knowledge into explainable, personalized recommendations.

## Primary outcome

A surfer can quickly decide where and when to surf and understand the evidence and uncertainty behind that recommendation.

## Initial user

The initial user is a technically capable surfer running the application in a personal homelab. Multi-user and hosted operation must remain possible without driving the first implementation.

## MVP capabilities

- create a surfer profile and preferences;
- save favorite surf spots;
- obtain marine and weather forecasts through replaceable providers;
- normalize and retain forecast provenance;
- score hourly conditions with deterministic rules;
- explain the recommendation and its uncertainty;
- cache recent recommendations in an installable mobile-first PWA;
- collect lightweight feedback after a surf session.

## Non-goals for the first release

- nautical navigation or safety certification;
- community discovery of secret or sensitive surf spots;
- autonomous AI decision-making;
- high-frequency real-time ocean monitoring;
- microservice decomposition;
- native App Store distribution;
- a general-purpose weather platform.

## Success signals

- A recommendation can be reproduced from stored inputs and a scoring version.
- The user can understand why two spots received different scores.
- A provider outage degrades the experience without corrupting stored data.
- A new forecast provider can be added without changing domain rules.
- The PWA remains useful when mobile connectivity is intermittent.
