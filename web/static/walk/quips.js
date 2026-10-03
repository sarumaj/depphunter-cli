// What the walker says to themselves, now and then: a line in a speech bubble at the
// side of the view when something happens - a bug bites or is caught, the parachute
// opens, a fall hurts, a ride is got on, a ball goes in - in the voice of the software
// engineer the walker is, hunting the bugs of their own codebase. Mixed into Walker
// (walk.js).
//
// Rarely enough to stay funny: one line at most every GAP, the same thing remarked on
// no more than once every TOPIC_GAP, and never the same line twice in a row.
//
// Implements: REQ-WALK-060

const GAP = 4;         // seconds between two lines at the least
const TOPIC_GAP = 20;  // ... and between two on the same thing
const READ = 4.5;      // seconds a line stays up, and ...
const PER_CHAR = 0.08; // ... a little longer for every character in it
const IDLE = 30;       // seconds stood stock still before the walker says so
const LOW = 0.3;       // the share of their health under which what hurts is remarked on as hurting

/** The lines, by what they are said about. */
export const QUIPS = {
  arrive: [
    'Deploying to the street. Fingers crossed.',
    'Hello, world.',
    'Logged in. Let\'s go find some bugs.',
    'Production. Smells like legacy code.',
    'git checkout street.',
    'Spinning up the environment.',
    'Works in staging. Let\'s see production.',
    'Running on prod. What could go wrong?',
    'Ready to triage.',
    'Another day, another stack trace.',
    'Time to pay down some tech debt.',
    'First rule: don\'t touch what works.',
  ],
  car: [
    'What the hell? Was this feature really required?',
    'A bobby car. Someone\'s sprint had slack in it.',
    'I\'d love to see the ticket for this one.',
    'Scope creep, now on four wheels.',
    'Who put this on the roadmap?',
    'Did product ask for this, or was it a hackathon?',
    'Somebody really wanted a demo for the all-hands.',
    'Ah yes, the feature nobody will maintain.',
    'Is this in the SLA?',
    'Nobody wrote a test for this, did they?',
    'Bobby car as a service. Bold.',
    'Must be the intern\'s side project.',
    'Ah, the MVP of transportation.',
  ],
  drive: [
    'Fine. Driving it. Strictly for QA purposes.',
    'No seatbelt. Classic MVP.',
    'Zero to production in under a second.',
    'Moving fast. Breaking things.',
    'Agile, in the literal sense.',
    'Steering by gut feeling. Like the roadmap.',
    'Vroom. Shipping velocity: high.',
    'Lane-keeping assist: not implemented.',
    'This car runs on legacy code.',
    'Driving straight into the next sprint.',
  ],
  swing: [
    'Testing the oscillation. Purely for science.',
    'Up, down, up, down. Like our error rate.',
    'This is what retries with backoff feel like.',
    'Feature flag on, feature flag off.',
    'Back and forth, like a code review.',
    'Swinging between tabs and spaces.',
    'Pendulum. Like my opinion on microservices.',
    'Exponential backoff, the fun version.',
  ],
  spin: [
    'Round and round, like a circular dependency.',
    'Infinite loop detected. Enjoying it.',
    'Spinning. Like the loading indicator on our dashboard.',
    'Polling. Still polling.',
    'Spinning up more instances.',
    'Event loop, but make it a ride.',
    'Round robin scheduling.',
    'I\'m the busy-wait now.',
  ],
  rock: [
    'Load balancing.',
    'Up and down, like the story points in planning.',
    'Rocking back and forth, like a flaky test.',
    'Blue, green, blue, green.',
    'A/B testing my balance.',
    'Merge conflict: up versus down.',
    'Eventually consistent.',
    'Optimistic locking. Literally rocking.',
  ],
  slide: [
    'Going down. Like prod on a Friday.',
    'Rollback in progress.',
    'Wheee - I mean, observing gravity.',
    'Downtime. Planned, this time.',
    'Smooth deploy. Suspiciously smooth.',
    'Fast path!',
    'Skipping the review. Wheee!',
    'Down the stack we go.',
  ],
  pipe: [
    'A pipeline! Finally, one that runs end to end.',
    'Entering the CI pipeline. Please don\'t fail.',
    'Now I know how a message in a queue feels.',
    'Stuck in the build queue.',
    'Through the pipeline, round the stages.',
    'Is this the message bus?',
    'Streaming. Real-time.',
    'Like a request through twelve middlewares.',
  ],
  score: [
    'Ship it!',
    'All tests green.',
    'Nothing but net. Like a clean merge.',
    'Put that in the changelog.',
    'Merged on the first try. Mark the date.',
    'Commit, push, done.',
    'Green build. Frame it.',
    'That\'s going in my performance review.',
    'Approved. No comments.',
    'Shipped before the deadline. A miracle.',
    'Hire me, I\'m a 10x baller.',
    'Coverage: 100 percent.',
    'Accepted answer, plus fifteen.',
  ],
  miss: [
    'Works on my machine.',
    'Off by one.',
    'That\'s a known issue.',
    'Out of bounds. Classic index error.',
    'Let\'s call it a feature.',
    'Flaky. Retry?',
    'Unexpected token.',
    'Undefined is not a function.',
    'Close enough for a demo.',
    'Needs more coffee.',
    'That was a test. Clearly.',
    'It passed locally.',
    'Blaming the physics engine.',
    'Marked as won\'t fix.',
  ],
  fly: [
    'Scaling vertically.',
    'Up we go, like the cloud bill.',
    'Now I\'m literally in the cloud.',
    'Who needs stairs with an unlimited budget?',
    'Elevating my privileges.',
    'Taking it to the next layer.',
    'Altitude: higher than our tech debt.',
    'Lift off. Like the AWS invoice.',
    'Cloud native, finally.',
    'Above the abstraction layer.',
    'Look, ma, no stairs.',
  ],
  empty: [
    'Running out of credits.',
    'Budget alert: 90 percent used.',
    'Quota exceeded. Of course.',
    'Out of memory.',
    'Somebody forgot to set a limit.',
    'Tank empty. Like the sprint backlog. Ha.',
    'Throttled by billing.',
  ],
  chute: [
    'Deploying the chute. Hopefully not the canary.',
    'Graceful degradation, engaged.',
    'Soft landing. Unlike our last release.',
    'Fallback mechanism engaged.',
    'Circuit breaker: open.',
    'Rollback plan: deployed.',
    'Safe mode on.',
    'Feature toggle: parachute.',
  ],
  landed: [
    'Landed. No incidents to report.',
    'Touchdown. No postmortem needed.',
    'Deployed safely. Nobody will believe it.',
    'Zero downtime. Nailed it.',
    'Rolled out without a rollback.',
    'Rolled out to 100 percent.',
    'Landed in prod. Monitoring.',
    'Smooth as a squash merge.',
  ],
  cutAway: [
    'Cutting the dependency. Let\'s see how that goes.',
    'Detached. Like the HEAD in my repo.',
    'Force-pushing to the ground.',
    'No safety net. Like main without tests.',
    'Detaching from upstream.',
    'Dropping the dependency. Bold.',
  ],
  wall: [
    'That was a hard dependency.',
    'Collision. Should have checked the hash.',
    'Hit the wall. Like every sprint.',
    'Unhandled exception: building.',
    'Firewall. Literal.',
    'Should\'ve validated input.',
  ],
  fall: [
    'Ouch. That\'s a breaking change.',
    'Should have read the docs on gravity.',
    'Gravity has no try/catch.',
    'Fall-through case. Somebody forgot the break.',
    'Stack overflow. Of me.',
    'That fall was not in the spec.',
    'Note to self: add a guard clause.',
    'Dropped. Like support for IE.',
    'Unexpected end of building.',
    'That\'s a 404: floor not found.',
  ],
  breath: [
    'Burned out. Need a sprint retro.',
    'Rate limited.',
    'Throttled. Like our API.',
    'Need a cooldown period.',
    'Heartbeat missed.',
    'Sprinting? We\'re agile, not crazy.',
    'My heart\'s at 100 percent CPU.',
    'Need to garbage collect.',
  ],
  bitten: [
    'Ow! It bit me. Reproduced, at least.',
    'Who left this in production?',
    'That\'s not a feature, that\'s a bite.',
    'I can confirm the bug exists.',
    'Steps to reproduce: stand here.',
    'It\'s not a bug, it\'s an undocumented bite.',
    'Regression confirmed. It hurts.',
    'QA was right all along.',
    'Who reviewed this?',
    'Production says hi.',
    'That\'s going in the incident log.',
    'A wild bug appeared!',
    'Ouch. Was that in the changelog?',
  ],
  critical: [
    'Critical! Page the on-call. Oh wait, that\'s me.',
    'Sev 1. Clear the calendar.',
    'P0! Drop everything!',
    'This one goes straight to the board.',
    'Wake up the CTO.',
    'War room. Now.',
    'All hands on deck. Mostly mine.',
  ],
  caught: [
    'Fixed. Closing the ticket.',
    'Caught one. Marking it resolved.',
    'Another one for the release notes.',
    'Works now. Don\'t ask why.',
    'Squashed. Coffee break earned.',
    'Root cause found. It was me.',
    'One less TODO.',
    'Closed as fixed. For real this time.',
    'Code review passed. Bug didn\'t.',
    'Patched.',
    'Bug 1, me 1. I\'m counting.',
    'Closed: works as intended now.',
    'That one had a long tail.',
    'Merged the fix. Nobody look.',
  ],
  burning: [
    'This is fine.',
    'Everything\'s on fire. A normal Monday.',
    'Hot path. Literally.',
    'Hotfix. Very hot.',
    'Deployed on a Friday, didn\'t you?',
    'Fire in the hole. Of the codebase.',
    'Server room vibes.',
    'Cooling not included.',
  ],
  water: [
    'In deep water. Like this codebase.',
    'Sinking. Like the legacy system.',
    'Memory leak. Water leak. Same thing.',
    'Data lake. Very literal.',
    'Going under, like an unpaid open source maintainer.',
    'Flooded. Like my inbox.',
    'Should\'ve installed the waterproof plugin.',
    'Glug. 503.',
  ],
  grapple: [
    'Pulling a dependency.',
    'git pull --force.',
    'Taking the shortcut. Tech debt, here I come.',
    'Linking against something taller.',
    'Hooked. Like on a new framework.',
    'Hooking into the lifecycle.',
    'Fetching remote origin.',
    'Dependency injection, the hard way.',
  ],
  hands: [
    'Hands free. Time to test the UX.',
    'Context switch. Tools down.',
    'Laptop closed. Mostly.',
    'Out of office. Kind of.',
    'Away from keyboard.',
    'No tools. Pure vibes.',
    'Hands off. Like the code freeze.',
  ],
  died: [
    'Segmentation fault (core dumped).',
    'Fatal exception. Restarting...',
    'Kernel panic.',
    'Blue screen. Literally.',
    'Process exited with code 1.',
    'Uncaught exception in main.',
    'Ctrl+Alt+Del.',
    'Exit code 137. Killed.',
    'Stack trace: everywhere.',
    'Rebooting the developer.',
    'Pod restarted.',
    'Unhandled promise rejection.',
  ],
  tag: [
    'Documented. Nobody will read it.',
    'Labeled. Like a ticket nobody picks up.',
    'Tagged. Release v0.0.1-whatever.',
    'Added to the backlog.',
    'Committed to memory. Mine, not git\'s.',
    'Bookmarked. Never to be opened again.',
    'Annotated. Very senior.',
  ],
  fireOut: [
    'Incident resolved. Writing the postmortem.',
    'Fire\'s out. Root cause: me, probably.',
    'Extinguished. Like my weekend plans.',
    'All green on the dashboard.',
    'Sprinkler deployed. Tests pass.',
    'Hot fix applied. Literally.',
  ],
  reach: [
    'Out of scope.',
    'Can\'t reach it. Needs a ticket.',
    'Out of range. Like the deadline.',
    '404: tool range not found.',
    'That\'s a cross-origin request.',
    'Needs a longer timeout.',
  ],
  rope: [
    'Out of rope. Out of runway.',
    'Line exhausted. Like the team.',
    'Ran out of buffer.',
    'End of stream.',
  ],
  skim: [
    'Walking on water. Senior engineer energy.',
    'Floating. Like my estimates.',
    'Serverless. Groundless, too.',
    'Floating point. Literally.',
  ],
  teleport: [
    'Context switch!',
    'Jumping to definition.',
    'Ctrl+click. Here I am.',
    'Hot reload!',
    'Deep link. Very deep.',
    'Jumped. Like a goto.',
  ],
  hurt: [
    'I need a sick day.',
    'My health check is failing.',
    'Degraded performance. Mine.',
    'Running on fumes and coffee.',
    'Error budget: exhausted.',
    'Status page: partial outage.',
    'I need a rollback.',
    'Rebooting soon, I fear.',
  ],
  allCaught: [
    'Zero open bugs. Screenshot it before someone files one.',
    'Inbox zero, bug edition.',
    'No known issues. Famous last words.',
    'Release candidate! Ship it!',
    'Clean build. Nobody touch anything.',
    '100 percent fixed. For now.',
  ],
  pickUp: [
    'Taking ownership of this ball.',
    'Assigned to me. Great.',
    'Got the ball. Now don\'t drop it.',
    'Checked out.',
    'Ball acquired. Lock held.',
    'Mine now. git blame says so.',
  ],
  idle: [
    'Waiting for the build...',
    'Still compiling.',
    'Is it frozen? Oh, it\'s me.',
    'Should I write a test for standing still?',
    'AFK. Back in five.',
    'Thinking. Or buffering.',
    'Reading the docs. Any minute now.',
    'Waiting on code review.',
    'In a meeting that could\'ve been an email.',
  ],
  scope: [
    'Zooming into the stack trace.',
    'Let me see the details.',
    'Enhance. Enhance!',
  ],
  photo: [
    'Screenshot for the bug report.',
    'Evidence. For the retro.',
    'Pics or it didn\'t happen.',
  ],
  wheel: [
    'So many tools. So little documentation.',
    'Choosing a framework. Again.',
    'Which tool is it this week?',
  ],
};

export const quips = {
  /**
   * Says something about `topic` (a key of QUIPS), if it is time to: always when
   * `always` (dying), otherwise only when nothing was said for GAP and nothing about
   * this for TOPIC_GAP.
   */
  quip(topic, always = false) {
    const lines = QUIPS[topic];
    if (!lines) return null;
    const now = performance.now() / 1000, said = this.said ||= { at: -Infinity, topics: {} };
    if (!always && (now - said.at < GAP || now - (said.topics[topic]?.at ?? -Infinity) < TOPIC_GAP)) return null;
    const last = said.topics[topic]?.line ?? -1;
    let line = Math.floor(Math.random() * lines.length);
    if (line === last && lines.length > 1) line = (line + 1) % lines.length;
    said.at = now;
    said.topics[topic] = { at: now, line };
    this.showQuip(lines[line]);
    return lines[line];
  },

  /** Says something about what hurt them - or, with little health left, about how they are. */
  ouch(topic) {
    const h = this.health;
    return this.quip(h && h.hp < h.max * LOW ? 'hurt' : topic);
  },

  /** A bug caught: remarked on, and the last one on the map most of all. */
  caughtOne() {
    const counts = this.bugs?.counts;
    if (counts && counts.total && counts.caught === counts.total) this.quip('allCaught', true);
    else this.quip('caught');
  },

  /** Every frame: a walker who has stood stock still for IDLE says so, once until they move. */
  fidget(deltaTime) {
    const p = this.p, at = `${p.x},${p.z},${p.yaw},${p.pitch}`;
    if (at !== this.stoodAt || this.keys?.size) {
      this.stoodAt = at;
      this.stoodFor = 0;
      return;
    }
    if ((this.stoodFor += deltaTime) >= IDLE && this.stoodFor - deltaTime < IDLE) this.quip('idle');
  },

  /** Puts `text` up in the bubble at the side of the view, and takes it down after a read. */
  showQuip(text) {
    const bubble = this.quipEl ||= this.hud?.querySelector('.w-quip');
    if (!bubble) return;
    bubble.textContent = text;
    bubble.hidden = false;
    bubble.classList.remove('gone');
    clearTimeout(this.quipTimer);
    this.quipTimer = setTimeout(() => {
      bubble.classList.add('gone');
      this.quipTimer = setTimeout(() => { bubble.hidden = true; }, 400);
    }, (READ + PER_CHAR * text.length) * 1000);
  },
};
