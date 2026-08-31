import React from "react";
import Link from "@docusaurus/Link";

import "./styles.css";

const GuideCard = (props) => {
  return (
    <Link className="guide-card-href" href={props.href}>
      <div className="guide-card-container">
        {/* Decorative only: the ordinal is generated in CSS and the link
            text carries the meaning. Hidden from assistive tech so the
            announced experience matches the visual one — font-size:0
            alone would leave the emoji in the accessibility tree. */}
        <div className="guide-card-icon" aria-hidden="true">
          {props.icon}
        </div>
        <div className="guide-card-detail">
          <div className="guide-card-title">{props.title}</div>
          <div className="guide-card-description">{props.description}</div>
        </div>
      </div>
    </Link>
  );
};

export default GuideCard;
