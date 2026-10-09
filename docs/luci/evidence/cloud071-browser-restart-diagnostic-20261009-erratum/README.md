# C071 documentation erratum

The frozen controlled C071 capsule is unchanged. Its README describes the cleanup drain order inaccurately. The installed and frozen helper sets `acceptRestartResponses=false` first, preventing acquisition of new response callbacks; then drains already accepted observations with the 5.5-second bound; then sets `closing=true` and begins ordinary bounded cleanup. This correction changes documentation only. The installed cold attempt used the original pinned helper bytes.

No request is replayed, no pending mutation is cleared on a refusal, and no late response status implies absence of an allocation. Valid 200 still requires exact identity and active health. This erratum neither promotes the original cold overall result nor claims its valid-200 run exercised the non-200 diagnostic branch.
