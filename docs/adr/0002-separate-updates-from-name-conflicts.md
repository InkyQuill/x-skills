# Separate updates from name conflicts

Remote install flows treat same-source archived skills as update candidates, but treat same-name skills without matching source metadata as name conflicts. This keeps routine updates efficient while forcing an explicit replace or rename decision when x-skills cannot prove the incoming skill is the same remote skill.
