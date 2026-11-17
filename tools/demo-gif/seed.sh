#!/bin/sh
# Makes the made-up "Pantry" project the board demo GIF is recorded on, in an empty folder.
#
#   tools/demo-gif/seed.sh /tmp/demo-proj
#
# It makes a git repository with a tiny plain-JavaScript meal planner (`node --test`, no dependencies) and the
# requirement the demo breaks down. It does not set Kanbanzai up: the recording does that with Init and Apply.
# The same folder name gives the same files every time (the commit's date and author are fixed).
set -eu

dir=${1:?usage: seed.sh EMPTY_FOLDER}
mkdir -p "$dir"
if [ -n "$(ls -A "$dir")" ]; then
	echo "seed.sh: $dir is not empty" >&2
	exit 1
fi
cd "$dir"

git init -q -b main
# The demo's owner is "Demo", whoever records it: Init reads the owner from git's user.name.
git config user.name Demo
git config user.email demo@example.com
mkdir src test

cat > package.json <<'EOF'
{
  "name": "pantry",
  "version": "0.1.0",
  "description": "A small meal planner",
  "private": true,
  "scripts": {
    "test": "node --test"
  }
}
EOF

cat > .gitignore <<'EOF'
node_modules/
EOF

cat > README.md <<'EOF'
# Pantry

A small meal planner: recipes in, a week of meals out.

    npm test
EOF

cat > src/recipes.js <<'EOF'
'use strict';

// A recipe is { name, servings, ingredients: [{ item, quantity, unit }] }.
const recipes = [
  {
    name: 'Tomato pasta',
    servings: 2,
    ingredients: [
      { item: 'pasta', quantity: 200, unit: 'g' },
      { item: 'tomatoes', quantity: 400, unit: 'g' },
      { item: 'garlic', quantity: 2, unit: 'cloves' },
    ],
  },
  {
    name: 'Veggie curry',
    servings: 4,
    ingredients: [
      { item: 'chickpeas', quantity: 400, unit: 'g' },
      { item: 'tomatoes', quantity: 400, unit: 'g' },
      { item: 'rice', quantity: 300, unit: 'g' },
    ],
  },
];

function find(name) {
  return recipes.find((recipe) => recipe.name.toLowerCase() === String(name).toLowerCase());
}

module.exports = { recipes, find };
EOF

cat > test/recipes.test.js <<'EOF'
'use strict';

const test = require('node:test');
const assert = require('node:assert');
const { find } = require('../src/recipes');

test('find a recipe by name, ignoring case', () => {
  assert.strictEqual(find('tomato PASTA').servings, 2);
});

test('find returns nothing for an unknown recipe', () => {
  assert.strictEqual(find('nothing'), undefined);
});
EOF

cat > REQUIREMENT.md <<'EOF'
# Plan a week of meals

People want to plan a week of meals from the recipes in Pantry, get one shopping list for the whole week, and share
the plan with the people they live with.

- Pick a recipe for each day, and change the number of servings for that day.
- The shopping list adds up the same ingredient across the week, in one unit, and leaves out what is already in the
  pantry.
- A plan can be shared as a link or a printed page, and the people it is shared with can tick items off the list.
EOF

git add -A
GIT_AUTHOR_NAME=Demo GIT_AUTHOR_EMAIL=demo@example.com GIT_AUTHOR_DATE='2026-01-01T00:00:00Z' \
GIT_COMMITTER_NAME=Demo GIT_COMMITTER_EMAIL=demo@example.com GIT_COMMITTER_DATE='2026-01-01T00:00:00Z' \
	git commit -q -m 'Pantry: recipes and a lookup'

echo "seeded $dir"
