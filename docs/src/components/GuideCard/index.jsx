import React from "react";
import { MDXProvider } from "@mdx-js/react";
import MDXComponents from "@theme/MDXComponents";
import Link from "@docusaurus/Link";

import "./styles.css";

const GuideCard = (props) => {
  const [displayLong, setDisplayLong] = React.useState(false);

  return (
    <Link className="guide-card-href" href={props.href}>
      <div className="guide-card-container">
        <div className="guide-card-description-short">
          <div className="guide-card-icon">
            <span className="fe fe-zap" />
            {props.icon}
          </div>
          <div className="guide-card-detail">
            <div className="guide-card-title">{props.title}</div>
            <div className="guide-card-description">
              {props.description}
              {React.Children.count(props.children) > 0 && (
                <span
                  className="guide-card-more fe fe-more-horizontal"
                  onClick={() => setDisplayLong(!displayLong)}
                />
              )}
            </div>
          </div>
        </div>
        {displayLong && (
          <div className="guide-card-description-long">
            <MDXProvider components={MDXComponents}>
              {props.children}
            </MDXProvider>
          </div>
        )}
      </div>
    </Link>
  );
};

export default GuideCard;
