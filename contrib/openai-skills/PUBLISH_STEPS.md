# Publish Steps (openai/skills)

1. Fork `openai/skills` on GitHub.
2. Clone your fork:
   - `git clone git@github.com:<your-user>/skills.git`
3. Create a branch:
   - `git checkout -b feat/add-relicta-release-governance-skill`
4. Copy the skill from this repo into your fork:
   - `cp -R <this-repo>/contrib/openai-skills/skills/.experimental/relicta-release-governance skills/.experimental/`
5. Validate in your fork:
   - `python3 scripts/quick_validate.py skills/.experimental/relicta-release-governance`
6. Commit:
   - `git add skills/.experimental/relicta-release-governance`
   - `git commit -m "feat(skills): add relicta-release-governance skill"`
7. Push:
   - `git push -u origin feat/add-relicta-release-governance-skill`
8. Open PR to `openai/skills:main` using text from:
   - `<this-repo>/contrib/openai-skills/PR_DESCRIPTION.md`
