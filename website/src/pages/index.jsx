import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import styles from './index.module.css';

const features = [
  {
    title: 'Track ownership',
    body: 'Tasks, claims, handoffs, reviews, evidence, and checkpoints live in one local execution store.'
  },
  {
    title: 'Keep lanes visible',
    body: 'The wall and board show what is claimed, what is actively attached to a provider session, and what needs review.'
  },
  {
    title: 'Stay provider-neutral',
    body: 'Codex, Claude, Gemini, tmux, or shell can attach to a lane without becoming Fairway-specific runtime dependencies.'
  }
];

function Feature({ title, body }) {
  return (
    <article className={styles.feature}>
      <h3>{title}</h3>
      <p>{body}</p>
    </article>
  );
}

export default function Home() {
  return (
    <Layout
      title="Coordination control plane for multi-agent engineering work"
      description="Fairway coordinates coding agents, worktrees, evidence, reviews, and sessions in one local-first control plane."
    >
      <main>
        <section className={styles.hero}>
          <div className={styles.heroInner}>
            <img src="/img/logo-lockup.svg" alt="Fairway" className={styles.logo} />
            <Heading as="h1">Coordination control plane for multi-agent engineering work</Heading>
            <p className={styles.lede}>
              Coordinate multiple coding agents working in parallel on one repository:
              task state, worktree lanes, provider sessions, evidence, handoffs,
              reviews, dashboard visibility, readiness, and workflow guards.
            </p>
            <div className={styles.actions}>
              <Link className={clsx('button button--primary button--lg', styles.primary)} to="/docs/quickstart">
                Start with the quickstart
              </Link>
              <Link className="button button--secondary button--lg" to="/docs/agent-guide">
                Read the agent guide
              </Link>
              <Link className="button button--secondary button--lg" to="/docs/design/product-boundaries">
                See product boundaries
              </Link>
            </div>
          </div>
        </section>
        <section className={styles.features}>
          {features.map((feature) => (
            <Feature key={feature.title} {...feature} />
          ))}
        </section>
        <section className={styles.install}>
          <Heading as="h2">Install</Heading>
          <pre><code>{'# tagged releases\nbrew tap fairway-run/tap\nbrew install --cask fairway\n\n# source install before the first tagged release\ngo install github.com/fairway-run/fairway/cmd/fairway@latest'}</code></pre>
        </section>
      </main>
    </Layout>
  );
}
