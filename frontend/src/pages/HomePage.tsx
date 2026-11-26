import { Link } from 'react-router-dom'

import {
  ArrowRight,
  ArrowUpRight,
  Code2,
  Workflow,
  Server,
  Layers,
  Play,
  Box,
  Zap,
  Terminal,
  Braces,
} from 'lucide-react'

import { FaGithub as Github } from 'react-icons/fa'

import './HomePage.css'

const GITHUB_URL =
  'https://github.com/Srinu0725/algotrace'

const features = [
  {
    icon: Workflow,
    title: 'See algorithms in action',
    description:
      'Explore how data evolves throughout the execution of your code.',
  },
  {
    icon: Code2,
    title: 'Your code, not predefined examples',
    description:
      'Write your own Python programs and explore their actual execution.',
  },
  {
    icon: Layers,
    title: 'Understand every step',
    description:
      'Inspect variables, function calls, execution states and changes.',
  },
  {
    icon: Play,
    title: 'Learn at your own pace',
    description:
      'Navigate execution snapshots forward and backward without losing context.',
  },
  {
    icon: Box,
    title: 'Isolated execution',
    description:
      'Run Python programs inside resource-limited Docker containers.',
  },
  {
    icon: Zap,
    title: 'Engineered for concurrency',
    description:
      'A Go-powered job queue and worker pool manage concurrent execution requests.',
  },
]

const architecture = [
  {
    icon: Code2,
    title: 'Interactive Workspace',
    tech: 'React · TypeScript · Monaco',
    description:
      'Write code and explore execution states in a synchronized workspace.',
  },
  {
    icon: Server,
    title: 'Execution API',
    tech: 'Go · REST · WebSockets',
    description:
      'Accept execution requests and deliver job status updates.',
  },
  {
    icon: Layers,
    title: 'Concurrent Job Engine',
    tech: 'Goroutines · Channels',
    description:
      'Queue and distribute execution jobs across concurrent workers.',
  },
  {
    icon: Box,
    title: 'Containerized Runtime',
    tech: 'Docker · Python 3.12',
    description:
      'Execute user programs inside resource-constrained environments.',
  },
  {
    icon: Braces,
    title: 'Execution Tracer',
    tech: 'Python · Structured JSON',
    description:
      'Capture variables, function calls and line-by-line execution events.',
  },
]

const previewCode = [
  'arr = [5, 3, 8, 1, 2]',
  '',
  'for i in range(len(arr)):',
  '    for j in range(len(arr)-i-1):',
  '        if arr[j] > arr[j+1]:',
  '            arr[j], arr[j+1] = (',
  '                arr[j+1], arr[j]',
  '            )',
  '',
  'print(arr)',
]

export default function HomePage() {
  return (
    <div className="home-page">
      <nav className="home-nav">
        <a href="#top" className="home-brand">
          <div className="home-logo">
            <Code2 size={23} />
          </div>

          <span>
            Algo<span>Trace</span>
          </span>
        </a>

        <div className="home-nav-links">
          <a href="#features">Features</a>
          <a href="#about">About</a>
          <a href="#architecture">
            Architecture
          </a>
        </div>

        <div className="home-nav-actions">
          <a
            href={GITHUB_URL}
            target="_blank"
            rel="noreferrer"
            className="home-github"
            aria-label="GitHub repository"
          >
            <Github size={20} />
          </a>

          <Link
            to="/workspace"
            className="home-button home-primary home-nav-cta"
          >
            Workspace
            <ArrowUpRight size={15} />
          </Link>
        </div>
      </nav>

      <main id="top">
        {/* Hero */}

        <section className="home-hero">
          <div className="home-eyebrow">
            <span className="home-live-dot" />
            BUILT FOR CURIOUS MINDS
          </div>

          <h1>
            Don't just write code.
            <br />
            <span>Watch it think.</span>
          </h1>

          <p className="home-hero-description">
            Turn code into an interactive visual
            experience. Explore algorithms,
            understand execution, and discover
            what happens beneath every line.
          </p>

          <div className="home-hero-actions">
            <Link
              to="/workspace"
              className="home-button home-primary"
            >
              <Play size={17} fill="currentColor" />
              Start Visualizing
              <ArrowRight size={17} />
            </Link>

            <a
              href={GITHUB_URL}
              target="_blank"
              rel="noreferrer"
              className="home-button home-secondary"
            >
              <Github size={18} />
              View Source
            </a>
          </div>

          <p className="home-hero-note">
            Real Python execution. Interactive traces.
            Built for learning.
          </p>
        </section>

        {/* Product preview */}

        <section className="home-preview">
          <div className="home-preview-window">
            <div className="home-window-header">
              <div className="home-window-dots">
                <i />
                <i />
                <i />
              </div>

              <span>AlgoTrace Workspace</span>

              <span className="home-preview-label">
                Preview
              </span>
            </div>

            <div className="home-preview-grid">
              <div className="home-preview-editor">
                <div className="home-preview-tab">
                  <Code2 size={15} />
                  main.py
                </div>

                <div className="home-preview-code">
                  {previewCode.map((line, index) => (
                    <div
                      key={index}
                      className={`home-code-line ${
                        index === 4
                          ? 'home-code-active'
                          : ''
                      }`}
                    >
                      <span>{index + 1}</span>
                      <code>{line || ' '}</code>
                    </div>
                  ))}
                </div>

                <div className="home-preview-console">
                  <Terminal size={15} />
                  Output Console
                </div>
              </div>

              <div className="home-preview-visual">
                <div className="home-preview-tab">
                  <Workflow size={15} />
                  Visualization
                </div>

                <div className="home-demo-content">
                  <div className="home-demo-title">
                    <span>arr</span>
                    <small>5 elements</small>
                  </div>

                  <div className="home-demo-bars">
                    {[3, 5, 8, 1, 2].map(
                      (value, index) => (
                        <div
                          className="home-demo-column"
                          key={index}
                        >
                          <span>{value}</span>

                          <div
                            className={`home-demo-bar ${
                              index < 2
                                ? 'home-demo-changed'
                                : ''
                            }`}
                            style={{
                              height: `${value * 20}px`,
                            }}
                          />

                          <small>{index}</small>
                        </div>
                      ),
                    )}
                  </div>

                  <div className="home-demo-variables">
                    <div>
                      <small>i</small>
                      <strong>0</strong>
                    </div>

                    <div>
                      <small>j</small>
                      <strong>1</strong>
                    </div>

                    <div>
                      <small>n</small>
                      <strong>5</strong>
                    </div>
                  </div>

                  <div className="home-demo-progress">
                    <span>Execution preview</span>

                    <div>
                      <i />
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <p className="home-preview-caption">
            A preview of the algorithm visualization
            experience.
          </p>
        </section>

        {/* Technology stack */}

        <section className="home-stack">
          <p>ENGINEERED WITH</p>

          <div>
            <span>Go</span>
            <span>Python</span>
            <span>Docker</span>
            <span>React</span>
            <span>TypeScript</span>
            <span>WebSockets</span>
          </div>
        </section>

        {/* Features */}

        <section
          id="features"
          className="home-section"
        >
          <div className="home-section-heading">
            <span className="home-kicker">
              THE EXPERIENCE
            </span>

            <h2>
              Understanding code,
              <br />
              <span>made visual.</span>
            </h2>

            <p>
              Everything you need to explore what
              your code is actually doing.
            </p>
          </div>

          <div className="home-feature-grid">
            {features.map((feature) => {
              const Icon = feature.icon

              return (
                <article
                  className="home-feature-card"
                  key={feature.title}
                >
                  <div className="home-feature-icon">
                    <Icon size={23} />
                  </div>

                  <h3>{feature.title}</h3>

                  <p>{feature.description}</p>
                </article>
              )
            })}
          </div>
        </section>

        {/* Personal story */}

        <section
          id="about"
          className="home-section home-story"
        >
          <div className="home-story-heading">
            <span className="home-kicker">
              WHY I BUILT THIS
            </span>

            <h2>
              Built from a problem
              <br />
              <span>I once faced.</span>
            </h2>

            <div className="home-story-quote">
              <span>“</span>

              The tool I wish I had when I started
              learning to code.
            </div>
          </div>

          <div className="home-story-content">
            <p>
              During my first year of B.Tech,
              when I started learning Data
              Structures and Algorithms, one of
              my biggest challenges wasn't writing
              code — it was visualizing what was
              happening behind it.
            </p>

            <p>
              I could read an algorithm, understand
              its syntax, and sometimes even solve
              the problem. But following how
              variables changed, how recursive
              calls unfolded, and how data moved
              through an algorithm was often
              difficult.
            </p>

            <p>
              I wanted to build something that
              would make this experience easier
              for others who face the same
              challenge.
            </p>

            <p>
              At the same time, my growing interest
              in backend engineering pushed me to
              explore execution engines, concurrent
              systems, containerized environments,
              and real-time communication.
            </p>

            <p className="home-story-emphasis">
              AlgoTrace is where these two
              interests come together.
            </p>

            <p>
              It's an attempt to bridge the gap
              between writing code and truly
              understanding it.
            </p>

            <div className="home-author">
              <div className="home-author-avatar">
                ST
              </div>

              <div>
                <strong>Srinivas Teleti</strong>
                <small>Creator of AlgoTrace</small>
              </div>
            </div>
          </div>
        </section>

        {/* Architecture */}

        <section
          id="architecture"
          className="home-section"
        >
          <div className="home-section-heading">
            <span className="home-kicker">
              UNDER THE HOOD
            </span>

            <h2>
              Simple experience.
              <br />
              <span>Real engineering.</span>
            </h2>

            <p>
              Behind every execution step is a
              system designed to run, trace and
              visualize real programs.
            </p>
          </div>

          <div className="home-architecture">
            {architecture.map((item, index) => {
              const Icon = item.icon

              return (
                <article
                  className="home-architecture-card"
                  key={item.title}
                >
                  <div className="home-architecture-number">
                    {String(index + 1).padStart(2, '0')}
                  </div>

                  <div className="home-feature-icon">
                    <Icon size={22} />
                  </div>

                  <div>
                    <h3>{item.title}</h3>
                    <span>{item.tech}</span>
                    <p>{item.description}</p>
                  </div>
                </article>
              )
            })}
          </div>
        </section>

        {/* Final call to action */}

        <section className="home-final-cta">
          <div className="home-feature-icon">
            <Code2 size={30} />
          </div>

          <h2>
            Your code.
            <br />
            <span>A whole new perspective.</span>
          </h2>

          <p>
            Whether you're learning your first
            sorting algorithm or exploring complex
            recursion, start by seeing what your
            code does.
          </p>

          <Link
            to="/workspace"
            className="home-button home-primary"
          >
            Launch Workspace
            <ArrowRight size={18} />
          </Link>
        </section>
      </main>

      <footer className="home-footer">
        <div>
          <strong>AlgoTrace</strong>
          <p>Built for learners, by a learner.</p>
        </div>

        <span>Designed and built by Srinivas Teleti.</span>

        <a
          href={GITHUB_URL}
          target="_blank"
          rel="noreferrer"
        >
          <Github size={18} />
          GitHub
        </a>
      </footer>
    </div>
  )
}
