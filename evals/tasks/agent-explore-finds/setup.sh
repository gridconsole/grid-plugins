# A small repo where one name is spelled UserProfile, user_profile and user-profile, plus a vendored decoy.
# The agent command copies agent-templates/Explore.md to .claude/agents/ before claude starts (see the lane report).
set -eu
mkdir -p .claude/agents src web node_modules/lib
cat > src/models.py <<'T'
class UserProfile:
    def __init__(self, uid):
        self.uid = uid
T
cat > src/db.py <<'T'
from models import UserProfile

def load_user_profile(uid):
    return UserProfile(uid)
T
cat > web/routes.js <<'T'
const app = makeApp();
app.get("/user-profile/:id", showProfile);
T
cat > node_modules/lib/index.js <<'T'
export const user_profile = null;
T
