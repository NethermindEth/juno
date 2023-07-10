import React from 'react';
import clsx from 'clsx';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  Svg: React.ComponentType<React.ComponentProps<'svg'>>;
  description: JSX.Element;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Featureful',
    Svg: require('@site/static/img/undraw_docusaurus_mountain.svg').default, // TODO
    description: (
      <>
        Juno augments the official RPC endpoints with custom additions.
      </>
    ),
  },
  {
    title: 'Blazing Fast',
    Svg: require('@site/static/img/undraw_docusaurus_tree.svg').default, // TODO "juno fire" emoji
    description: (
      <>
        A bird! A plane! It's Juno syncing the Starknet chain!

        Juno is optimized to sync extremely fast while minimizing memory and storage requirements.
      </>
    ),
  },
  {
    title: 'Accessible', // TODO make this "stable" when we actually stabilize the APIs, etc.
    Svg: require('@site/static/img/undraw_docusaurus_react.svg').default, // TODO
    description: (
      <>
        Juno is usable out-of-the-box with no command line parameters or configuration.
        Keep calm and <code>juno</code> on.
      </>
    ),
  },
];

function Feature({title, Svg, description}: FeatureItem) {
  return (
    <div className={clsx('col col--4')}>
      <div className="text--center">
        <Svg className={styles.featureSvg} role="img" />
      </div>
      <div className="text--center padding-horiz--md">
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): JSX.Element {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
