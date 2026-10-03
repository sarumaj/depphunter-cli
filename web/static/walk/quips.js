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

const GAP = 10;        // seconds between two lines at the least
const TOPIC_GAP = 45;  // ... and between two on the same thing
const READ = 2.6;      // seconds a line stays up, and ...
const PER_CHAR = 0.05; // ... a little longer for every character in it

/** The lines, by what they are said about. */
export const QUIPS = {
  arrive: [
    'Deploying to the street. Fingers crossed.',
    'Hello, world.',
    'Logged in. Let\'s go find some bugs.',
    'Production. Smells like legacy code.',
  ],
  car: [
    'What the hell? Was this feature really required?',
    'A bobby car. Someone\'s sprint had slack in it.',
    'I\'d love to see the ticket for this one.',
    'Scope creep, now on four wheels.',
    'Who put this on the roadmap?',
  ],
  drive: [
    'Fine. Driving it. Strictly for QA purposes.',
    'No seatbelt. Classic MVP.',
    'Zero to production in under a second.',
  ],
  swing: [
    'Testing the oscillation. Purely for science.',
    'Up, down, up, down. Like our error rate.',
    'This is what retries with backoff feel like.',
  ],
  spin: [
    'Round and round, like a circular dependency.',
    'Infinite loop detected. Enjoying it.',
    'Spinning. Like the loading indicator on our dashboard.',
  ],
  rock: [
    'Load balancing.',
    'Up and down, like the story points in planning.',
    'Rocking back and forth, like a flaky test.',
  ],
  slide: [
    'Going down. Like prod on a Friday.',
    'Rollback in progress.',
    'Wheee - I mean, observing gravity.',
  ],
  pipe: [
    'A pipeline! Finally, one that runs end to end.',
    'Entering the CI pipeline. Please don\'t fail.',
    'Now I know how a message in a queue feels.',
  ],
  score: [
    'Ship it!',
    'All tests green.',
    'Nothing but net. Like a clean merge.',
    'Put that in the changelog.',
    'Merged on the first try. Mark the date.',
  ],
  miss: [
    'Works on my machine.',
    'Off by one.',
    'That\'s a known issue.',
    'Out of bounds. Classic index error.',
    'Let\'s call it a feature.',
  ],
  fly: [
    'Scaling vertically.',
    'Up we go, like the cloud bill.',
    'Now I\'m literally in the cloud.',
    'Who needs stairs with an unlimited budget?',
  ],
  empty: [
    'Running out of credits.',
    'Budget alert: 90 percent used.',
    'Quota exceeded. Of course.',
  ],
  chute: [
    'Deploying the chute. Hopefully not the canary.',
    'Graceful degradation, engaged.',
    'Soft landing. Unlike our last release.',
  ],
  landed: [
    'Landed. No incidents to report.',
    'Touchdown. No postmortem needed.',
    'Deployed safely. Nobody will believe it.',
  ],
  cutAway: [
    'Cutting the dependency. Let\'s see how that goes.',
    'Detached. Like the HEAD in my repo.',
  ],
  wall: [
    'That was a hard dependency.',
    'Collision. Should have checked the hash.',
  ],
  fall: [
    'Ouch. That\'s a breaking change.',
    'Should have read the docs on gravity.',
    'Gravity has no try/catch.',
    'Fall-through case. Somebody forgot the break.',
  ],
  breath: [
    'Burned out. Need a sprint retro.',
    'Rate limited.',
    'Throttled. Like our API.',
  ],
  bitten: [
    'Ow! It bit me. Reproduced, at least.',
    'Who left this in production?',
    'That\'s not a feature, that\'s a bite.',
    'I can confirm the bug exists.',
    'Steps to reproduce: stand here.',
  ],
  critical: [
    'Critical! Page the on-call. Oh wait, that\'s me.',
    'Sev 1. Clear the calendar.',
  ],
  caught: [
    'Fixed. Closing the ticket.',
    'Caught one. Marking it resolved.',
    'Another one for the release notes.',
    'Works now. Don\'t ask why.',
    'Squashed. Coffee break earned.',
    'Root cause found. It was me.',
  ],
  burning: [
    'This is fine.',
    'Everything\'s on fire. A normal Monday.',
    'Hot path. Literally.',
  ],
  water: [
    'In deep water. Like this codebase.',
    'Sinking. Like the legacy system.',
    'Memory leak. Water leak. Same thing.',
  ],
  grapple: [
    'Pulling a dependency.',
    'git pull --force.',
    'Taking the shortcut. Tech debt, here I come.',
  ],
  hands: [
    'Hands free. Time to test the UX.',
    'Context switch. Tools down.',
  ],
  died: [
    'Segmentation fault (core dumped).',
    'Fatal exception. Restarting...',
    'Kernel panic.',
    'Blue screen. Literally.',
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
