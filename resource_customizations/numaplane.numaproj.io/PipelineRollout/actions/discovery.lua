local actions = {}
actions["pause"] = {["disabled"] = true}
actions["unpause"] = {["disabled"] = true}

local paused = false
if obj.spec.pipeline.spec.lifecycle.desiredPhase == "Paused" then
  paused = true
end
if paused then
  actions["unpause"]["disabled"] = false
else
  actions["pause"]["disabled"] = false
end
return actions