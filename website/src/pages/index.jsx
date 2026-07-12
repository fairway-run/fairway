import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import styles from './index.module.css';

const accountability = [
  { step: '01', title: 'Declare intent', body: 'Define the task, scope, risk, owner, and authority boundary before implementation starts.' },
  { step: '02', title: 'Record decisions', body: 'Keep material choices and deviations durable without treating provider transcripts as provenance.' },
  { step: '03', title: 'Attach evidence', body: 'Link tests, CI, environment proof, and artifacts to the work they support.' },
  { step: '04', title: 'Apply judgment', body: 'Route independent review according to risk and retain findings, not just approval status.' },
  { step: '05', title: 'Promote explicitly', body: 'Make merge, release, deploy, and live-operation boundaries visible without granting them to Fairway.' }
];

function AccountabilityStep({ step, title, body }) {
  return (
    <article className={styles.step}>
      <span className={styles.stepNumber}>{step}</span>
      <h3>{title}</h3>
      <p>{body}</p>
    </article>
  );
}

export default function Home() {
  return (
    <Layout
      title="Engineering control for agent-driven delivery"
      description="Fairway is the independent engineering record and control layer for accountable intent, decisions, evidence, review, and promotion in agent-driven delivery."
    >
      <main>
        <section className={styles.hero}>
          <div className={styles.heroShade} aria-hidden="true" />
          <div className={styles.heroInner}>
            <img src="/img/logo-lockup.svg" alt="Fairway" className={styles.logo} />
            <Heading as="h1">Fairway</Heading>
            <p className={styles.kicker}>Engineering control for agent-driven delivery</p>
            <p className={styles.lede}>
              Keep accountable intent, material decisions, evidence,
              independent judgment, and promotion state durable when coding
              agents and provider sessions change.
            </p>
            <div className={styles.actions}>
              <Link className={clsx('button button--primary button--lg', styles.primary)} to="/docs/quickstart">
                Complete one bounded task
              </Link>
              <Link className="button button--secondary button--lg" to="/docs/product">
                Evaluate Fairway
              </Link>
            </div>
            <p className={styles.heroBoundary}>
              Local-first and provider-neutral. Fairway records control state;
              it does not become your source-control, CI, identity, or deployment authority.
            </p>
          </div>
        </section>

        <section className={styles.accountability}>
          <div className={styles.sectionHeading}>
            <span>One accountability chain</span>
            <Heading as="h2">From declared intent to explicit promotion</Heading>
            <p>
              Fairway connects the facts that other systems produce without
              absorbing their authority.
            </p>
          </div>
          <div className={styles.steps}>
            {accountability.map((item) => <AccountabilityStep key={item.step} {...item} />)}
          </div>
        </section>

        <section className={styles.productProof}>
          <div className={styles.proofCopy}>
            <span>Operational readback</span>
            <Heading as="h2">See work, waits, evidence, and review state without turning the dashboard into a control console.</Heading>
            <p>
              The read-only dashboard projects the same Fairway record used by
              the CLI. Mutating work stays in trusted command and provider
              surfaces with explicit policy checks.
            </p>
            <div className={styles.textLinks}>
              <Link to="/docs/design/dashboard">Explore the dashboard model</Link>
              <Link to="/docs/case-studies/ai-cloud">Read the internal AI Cloud case study</Link>
            </div>
          </div>
          <figure className={styles.dashboardFigure}>
            <img src="/img/dashboard/fairway-dashboard-board.png" alt="Fairway board showing gate readiness, workstream progress, task filters, and activity" />
            <figcaption>Actual Fairway board UI; dashboard views remain read-only at shared trust boundaries.</figcaption>
          </figure>
        </section>

        <section className={styles.startBand}>
          <div>
            <span>First value in minutes</span>
            <Heading as="h2">Start with one bounded result, not the whole operating model.</Heading>
            <p>
              Initialize Fairway in a clean repository, start one task, attach
              passing evidence, verify the boundary, and close the work. Add
              lanes, watchers, shared views, and policy profiles only when the
              project needs them.
            </p>
          </div>
          <div className={styles.install}>
            <div className={styles.installHeader}>
              <strong>Install the current release</strong>
              <Link to="/docs/quickstart">Open the five-minute path</Link>
            </div>
            <pre><code>{'brew tap fairway-run/tap\nbrew install --cask fairway\nfairway version'}</code></pre>
          </div>
        </section>

        <section className={styles.authority}>
          <div>
            <span>Authority boundary</span>
            <Heading as="h2">Evidence informs promotion. It never silently grants it.</Heading>
          </div>
          <p>
            Coding agents implement. Reviewers judge. Source control merges.
            CI verifies. Identity systems authenticate. Operators deploy and
            run live actions. Fairway makes the handoffs and proof accountable.
          </p>
          <Link className="button button--secondary" to="/docs/design/product-boundaries">
            Read the product boundaries
          </Link>
        </section>
      </main>
    </Layout>
  );
}
