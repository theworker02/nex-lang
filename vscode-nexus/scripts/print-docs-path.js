#!/usr/bin/env node
/** Print the absolute path to the Nexus docs tree. */
const path = require("path");
const docs = path.resolve(__dirname, "..", "docs");
console.log("Nexus docs:", docs);
console.log("Start at:", path.join(docs, "README.md"));
