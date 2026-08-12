import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import styles from './index.module.css';

const accountability = [
  { step: '01', title: 'Collaborate', body: 'Discover the product need, constraints, owning boundary, and proof while uncertainty remains.' },
  { step: '02', title: 'Bound', body: 'Name the owner, dependencies, acceptance, risk, and independently checkable result.' },
  { step: '03', title: 'Delegate', body: 'Attach one provider, subagent, thread, or utility to execute the bounded slice.' },
  { step: '04', title: 'Verify', body: 'Link reproducible tests, CI, environment proof, and safe artifacts to the claim.' },
  { step: '05', title: 'Challenge', body: 'Let independent review or conflicting evidence reopen the diagnosis when needed.' },
  { step: '06', title: 'Promote', body: 'Cross merge, release, deploy, or live boundaries only through the authority that owns them.' }
];

const productLayers = [
  {
    index: '01',
    name: 'Work coordination',
    status: 'Implemented',
    summary: 'Keep tasks, lanes, worktrees, sessions, ownership, waits, and next actions durable across agent runs.',
    detail: 'Tasks · lanes · worktrees · sessions · readiness'
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
      title="Collaborative problem-solving with governed delegation"
      description="Fairway preserves the durable control record as engineering work moves between collaboration, bounded delegation, independent evidence, and explicit promotion."
    >
      <main>
        <section className={styles.hero}>
          <div className={styles.heroShade} aria-hidden="true" />
          <div className={styles.heroInner}>
            <img src="/img/logo-lockup.svg" alt="Fairway" className={styles.logo} />
            <Heading as="h1">Fairway</Heading>
            <p className={styles.kicker}>Collaborative problem-solving · governed delegation</p>
            <p className={styles.lede}>
              Collaborate while the problem and proof are uncertain. Delegate
              one bounded slice when its result can be independently checked.
              Fairway keeps the decisions, sessions, evidence, reviews, and
              authority boundaries durable across every run.
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
              Coding agents and optional Seaway execute runs. Git, CI/CD,
              trackers, and identity systems retain their own authority.
            </p>
          </div>
        </section>

        <section className={styles.productModel}>
          <div className={styles.modelHeading}>
            <div>
              <span>One durable control record</span>
              <Heading as="h2">Move between collaboration and delegation without losing the engineering story.</Heading>
            </div>
            <p>
              The model operates across architecture, implementation, review,
              CI/CD, deployment, and operations. It is not another SDLC phase,
              and it does not treat prompt length as task readiness.
            </p>
          </div>
          <div className={styles.productLayers}>
            {productLayers.map((item) => <ProductLayer key={item.index} {...item} />)}
          </div>
          <div className={styles.modelLinks}>
            <Link to="/docs/design/collaborative-delegation">How collaboration becomes safe delegation</Link>
            <Link to="/docs/design/project-working-memory">How track memory works</Link>
            <Link to="/docs/design/engineering-knowledge">How engineering knowledge stays grounded</Link>
            <Link to="/docs/design/rule-packs">How reusable rules compose</Link>
            <Link to="/docs/quality-record-demo">Run the Quality Record demo</Link>
          </div>
        </section>

        <section className={styles.systemMap}>
          <div className={styles.sectionHeading}>
            <span>The system boundary</span>
            <Heading as="h2">Coordinate the work without absorbing the systems that perform it.</Heading>
            <p>
              Fairway connects durable work to zero or more execution runs.
              Facts cross the boundary; task, review, and promotion authority do not.
            </p>
          </div>
          <div className={styles.mapFlow} aria-label="Fairway system relationship">
            <article className={styles.mapPrimary}>
              <strong>Fairway</strong>
              <span>Tasks · lanes · worktrees · sessions · reviews · evidence · readiness</span>
            </article>
            <div className={styles.mapConnector}>bounded context ↓ correlated facts ↑</div>
            <article>
              <strong>Coding agents + optional Seaway</strong>
              <span>Individual run admission · execution · policy · events · results · usage</span>
            </article>
            <div className={styles.mapConnector}>run execution ↓</div>
            <article>
              <strong>Models · tools · execution environments</strong>
              <span>Provider interaction · filesystem · network · runtime capabilities</span>
            </article>
          </div>
          <div className={styles.authorityGrid}>
            <span><strong>Git and forges</strong> own source and merge</span>
            <span><strong>CI/CD</strong> owns verification and deployment</span>
            <span><strong>Trackers</strong> own stakeholder planning</span>
            <span><strong>Identity systems</strong> own authentication and access</span>
          </div>
          <Link to="/docs/ecosystem">See the complete ecosystem boundary</Link>
        </section>

        <section className={styles.accountability}>
          <div className={styles.sectionHeading}>
            <span>The working loop</span>
            <Heading as="h2">Collaborate on the problem. Delegate the bounded work.</Heading>
            <p>
              Verification cost—not model capability—sets the delegation
              boundary. Evidence can challenge the result and return the work
              to collaboration before promotion.
            </p>
          </div>
          <div className={styles.steps}>
            {accountability.map((item) => <AccountabilityStep key={item.step} {...item} />)}
          </div>
        </section>

        <section className={styles.productProof}>
          <div className={styles.proofCopy}>
            <span>Operational readback</span>
            <Heading as="h2">Inspect the cited Quality Record without turning the dashboard into an approval console.</Heading>
            <p>
              The read-only dashboard projects the same intent, evidence,
              verification, judgment, promotion, outcome, and lesson states as
              the CLI. Missing, unavailable, conflicting, and externally owned
              facts stay visible instead of becoming a generated score.
            </p>
            <div className={styles.textLinks}>
              <Link to="/docs/design/dashboard">Explore the dashboard model</Link>
              <Link to="/docs/quality-record-demo">Run the ten-minute demonstration</Link>
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
