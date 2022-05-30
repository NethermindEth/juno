import React from 'react';
import clsx from 'clsx';
import styles from './styles.module.css';

const FeatureList = [
    {
        title: 'Starknet in your hands',
        Svg: require('@site/static/img/starknet.svg').default,
        description: (
            <>
                You can get all the Starknet information directly from us.
            </>
        ),
    },
    {
        title: 'Oriented to Services',
        Svg: require('@site/static/img/cloud.svg').default,
        description: (
            <>
                Each of the parts of the application is oriented to services,
            </>
        ),
    },
    {
        title: 'Powered by Golang',
        Svg: require('@site/static/img/go_saiyan.svg').default,
        description: (
            <>
                Go made it easy for us to write maintainable, testable, lightweight and performant code.
            </>
        ),
    },
];

function Feature({Svg, title, description}) {
    return (
        <div className={clsx('col col--4')}>
            <div className="text--center">
                <Svg className={styles.featureSvg} role="img"/>
            </div>
            <div className="text--center padding-horiz--md">
                <h3>{title}</h3>
                <p>{description}</p>
            </div>
        </div>
    );
}

export default function HomepageFeatures() {
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
