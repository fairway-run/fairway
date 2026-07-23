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

const productLayers = [
  {
    index: '01',
    name: 'Execution control',
    status: 'Implemented',
    summary: 'Bound work to durable intent, ownership, decisions, evidence, review, and promotion state.',
    detail: 'Tasks · sessions · checkpoints · evidence · reviews'
  },
  {
    index: '02',
    name: 'Engineering continuity',
    status: 'Implemented',
    summary: 'Resume after context loss or provider replacement without making chat history the system of record.',
    detail: 'Track memory · cold starts · handoffs · waits'
  },
  {
    index: '03',
    name: 'Operating knowledge',
    status: 'Implemented',
    summary: 'Carry source-grounded project knowledge and reusable engineering rules across work items.',
    detail: 'Knowledge packets · rule packs · provenance · freshness'
  },
  {
    index: '04',
    name: 'Assurance and profiles',
    status: 'Implemented + planned',
    summary: 'Map recorded facts to bounded readiness claims, then compose them into specialized execution profiles.',
    detail: 'Assurance profiles implemented · migration profile planned'
  }
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

function ProductLayer({ index, name, status, summary, detail }) {
  return (
    <article className={styles.productLayer}>
      <span className={styles.layerIndex}>{index}</span>
      <div className={styles.layerName}>
        <h3>{name}</h3>
        <span>{status}</span>
      </div>
      <p>{summary}</p>
      <strong>{detail}</strong>
    </article>
  );
}

export default function Home() {
  return (
    <Layout
      title="Engineering continuity and control for agent-driven delivery"
      description="Fairway keeps agent-driven engineering governed, resumable, and reviewable through durable execution records, memory, knowledge, rules, and assurance."
    >
      <main>
        <section className={styles.hero}>
          <div className={styles.heroShade} aria-hidden="true" />
          <div className={styles.heroInner}>
            <img src="/img/logo-lockup.svg" alt="Fairway" className={styles.logo} />
            <Heading as="h1">Fairway</Heading>
            <p className={styles.kicker}>Engineering continuity and control</p>
            <p className={styles.lede}>
              Keep agent-driven engineering governed, resumable, and
              reviewable across providers, tools, and time. Fairway connects
              execution records, working memory, project knowledge, reusable
              rules, and promotion evidence without taking authority from the
              systems that perform the work.
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

        <section className={styles.productModel}>
          <div className={styles.modelHeading}>
            <div>
              <span>One durable engineering record</span>
              <Heading as="h2">Control today&apos;s work. Preserve what the next agent needs.</Heading>
            </div>
            <p>
              Fairway begins with accountable execution and extends into
              continuity, operating knowledge, and evidence-backed assurance.
              Each layer has a distinct job and an explicit authority boundary.
            </p>
          </div>
          <div className={styles.productLayers}>
            {productLayers.map((item) => <ProductLayer key={item.index} {...item} />)}
          </div>
          <div className={styles.modelLinks}>
            <Link to="/docs/design/project-working-memory">How track memory works</Link>
            <Link to="/docs/design/engineering-knowledge">How engineering knowledge stays grounded</Link>
            <Link to="/docs/design/rule-packs">How reusable rules compose</Link>
          </div>
        </section>

        <section className={styles.accountability}>
          <div className={styles.sectionHeading}>
            <span>The control path</span>
            <Heading as="h2">From declared intent to explicit promotion</Heading>
            <p>
              The same accountability chain applies whether one agent handles
              a small fix or several providers execute a long-running program.
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
              the CLI, memory packets, and knowledge queries. Mutating work
              stays in trusted command and provider surfaces with explicit
              policy checks.
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
